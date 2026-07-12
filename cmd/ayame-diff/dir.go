package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/hjosugi/ayame-diff/internal/dircompare"
	"github.com/hjosugi/ayame-diff/internal/engine"
)

// runDir implements: ayame-diff dir [flags] OLD_DIR NEW_DIR
func runDir(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff dir", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var all, jsonOut, tsvOut, includeHidden, quick, diffExit bool
	excludes, includes := stringFlags(), stringFlags()
	workers := min(runtime.NumCPU(), 8)
	maxArchiveEntryBytes, maxArchiveBytes := "64MiB", "256MiB"
	fs.BoolVar(&all, "all", false, "include unchanged (same) files in the output")
	fs.BoolVar(&jsonOut, "json", false, "emit the result as JSON")
	fs.BoolVar(&tsvOut, "tsv", false, "emit status/path/size/mtime as TSV")
	fs.Var(&excludes, "exclude", "glob to skip (repeatable), matched on the relative path or base name")
	fs.Var(&includes, "include", "glob to include (repeatable), matched on the relative path or base name")
	fs.BoolVar(&includeHidden, "hidden", false, "include dotfiles and hidden dot-directories (symlinks are always skipped)")
	fs.BoolVar(&quick, "quick", false, "trust equal size+mtime without reading content")
	fs.IntVar(&workers, "workers", workers, "parallel content comparison workers (1..64)")
	fs.StringVar(&maxArchiveEntryBytes, "max-archive-entry-bytes", maxArchiveEntryBytes, "maximum uncompressed size of one archive entry")
	fs.StringVar(&maxArchiveBytes, "max-archive-bytes", maxArchiveBytes, "maximum total uncompressed size of one archive")
	fs.BoolVar(&diffExit, "diff-exit-code", false, "exit 1 when differences exist; errors exit 2")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff dir [flags] OLD NEW

Recursively compare two directory trees, or archives (.zip/.tar/.tar.gz/.tgz),
by file content. Reports files that were added (+), removed (-), or changed (~).
Unchanged files are hidden unless --all is given.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "error: dir needs exactly two paths: OLD NEW (directories or archives)")
		return 2
	}

	if jsonOut && tsvOut {
		fmt.Fprintln(stderr, "error: --json and --tsv cannot be combined")
		return 2
	}
	if workers < 1 || workers > 64 {
		fmt.Fprintln(stderr, "error: --workers must be from 1 to 64")
		return 2
	}
	entryLimit, err := parsePositiveByteSize("--max-archive-entry-bytes", maxArchiveEntryBytes)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	totalLimit, err := parsePositiveByteSize("--max-archive-bytes", maxArchiveBytes)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if entryLimit > totalLimit {
		fmt.Fprintln(stderr, "error: --max-archive-entry-bytes cannot exceed --max-archive-bytes")
		return 2
	}
	res, err := dircompare.CompareAny(fs.Arg(0), fs.Arg(1), dircompare.Options{
		Excludes: excludes.values, Includes: includes.values, IncludeHidden: includeHidden,
		Quick: quick, Workers: workers, MaxArchiveEntryBytes: entryLimit, MaxArchiveBytes: totalLimit,
	})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	if jsonOut {
		if err := writeDirJSON(stdout, res); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
	} else if tsvOut {
		if err := writeDirTSV(stdout, res, all); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
		fmt.Fprintf(stderr, "%d added, %d removed, %d changed, %d same\n", res.Added, res.Removed, res.Changed, res.Same)
	} else {
		if err := writeDirText(stdout, stderr, res, all); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
	}
	if diffExit && res.Added+res.Removed+res.Changed > 0 {
		return 1
	}
	return 0
}

func parsePositiveByteSize(flagName, value string) (int64, error) {
	size, err := engine.ParseByteSize(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", flagName, err)
	}
	if size < 1 {
		return 0, fmt.Errorf("%s must be at least 1 byte", flagName)
	}
	return size, nil
}

func writeDirTSV(stdout io.Writer, res *dircompare.Result, all bool) error {
	bw := bufio.NewWriter(stdout)
	fmt.Fprintln(bw, "status\tpath\told_size\tnew_size\told_mtime\tnew_mtime")
	for _, entry := range res.Entries {
		if entry.Status == dircompare.Same && !all {
			continue
		}
		fmt.Fprintf(bw, "%s\t%s\t%d\t%d\t%s\t%s\n", entry.Status, entry.Path, entry.OldSize, entry.NewSize, formatDirTime(entry.OldModTime), formatDirTime(entry.NewModTime))
	}
	return bw.Flush()
}

func formatDirTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

var dirMarker = map[dircompare.Status]byte{
	dircompare.Added:   '+',
	dircompare.Removed: '-',
	dircompare.Changed: '~',
	dircompare.Same:    '=',
}

func writeDirText(stdout, stderr io.Writer, res *dircompare.Result, all bool) error {
	bw := bufio.NewWriter(stdout)
	for _, e := range res.Entries {
		if e.Status == dircompare.Same && !all {
			continue
		}
		fmt.Fprintf(bw, "%c %s\n", dirMarker[e.Status], e.Path)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "%d added, %d removed, %d changed, %d same\n",
		res.Added, res.Removed, res.Changed, res.Same)
	return nil
}

type dirEntryJSON struct {
	Path     string `json:"path"`
	Status   string `json:"status"`
	OldSize  int64  `json:"old_size"`
	NewSize  int64  `json:"new_size"`
	OldMTime string `json:"old_mtime,omitempty"`
	NewMTime string `json:"new_mtime,omitempty"`
}

func writeDirJSON(stdout io.Writer, res *dircompare.Result) error {
	entries := make([]dirEntryJSON, len(res.Entries))
	for i, e := range res.Entries {
		entries[i] = dirEntryJSON{Path: e.Path, Status: e.Status.String(), OldSize: e.OldSize, NewSize: e.NewSize, OldMTime: formatDirTime(e.OldModTime), NewMTime: formatDirTime(e.NewModTime)}
	}
	out := struct {
		Added   int            `json:"added"`
		Removed int            `json:"removed"`
		Changed int            `json:"changed"`
		Same    int            `json:"same"`
		Entries []dirEntryJSON `json:"entries"`
	}{res.Added, res.Removed, res.Changed, res.Same, entries}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(data, '\n'))
	return err
}
