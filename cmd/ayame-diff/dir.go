package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hjosugi/ayame-diff/internal/atomicfile"
	"github.com/hjosugi/ayame-diff/internal/dircompare"
	"github.com/hjosugi/ayame-diff/internal/dirreport"
	"github.com/hjosugi/ayame-diff/internal/engine"
)

// runDir implements: ayame-diff dir [flags] LEFT_DIR RIGHT_DIR
func runDir(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff dir", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var all, jsonOut, tsvOut, includeHidden, quick, diffExit bool
	var htmlPath, csvPath, filterExpression, filterFile, compareBy string
	var listFilterSets bool
	excludes, includes, filterSets := stringFlags(), stringFlags(), stringFlags()
	workers := min(runtime.NumCPU(), 8)
	maxArchiveEntryBytes, maxArchiveBytes := "64MiB", "256MiB"
	fs.BoolVar(&all, "all", false, "include unchanged (same) files in the output")
	fs.BoolVar(&jsonOut, "json", false, "emit the result as JSON")
	fs.BoolVar(&tsvOut, "tsv", false, "emit status/path/size/mtime as TSV")
	fs.StringVar(&htmlPath, "html", "", "write a self-contained folder tree report")
	fs.StringVar(&csvPath, "csv", "", "write an RFC 4180 folder summary")
	fs.Var(&excludes, "exclude", "glob to skip (repeatable), matched on the relative path or base name")
	fs.Var(&includes, "include", "glob to include (repeatable), matched on the relative path or base name")
	fs.StringVar(&filterExpression, "filter", "", "filter expression (size/name/path/ext/mtime with and/or/not)")
	fs.StringVar(&filterFile, "filter-file", "", "JSON named-filter set or .ayamediff.json project")
	fs.Var(&filterSets, "filter-set", "built-in or file-defined filter set (repeatable)")
	fs.BoolVar(&listFilterSets, "list-filter-sets", false, "list bundled filter-set names and exit")
	fs.BoolVar(&includeHidden, "hidden", false, "include dotfiles and hidden dot-directories (symlinks are always skipped)")
	fs.StringVar(&compareBy, "compare-by", "", "comparison method: contents, quick, hash, date, or size (default contents)")
	fs.BoolVar(&quick, "quick", false, "alias for --compare-by quick")
	fs.IntVar(&workers, "workers", workers, "parallel content comparison workers (1..64)")
	var maxEntries int
	fs.StringVar(&maxArchiveEntryBytes, "max-archive-entry-bytes", maxArchiveEntryBytes, "maximum uncompressed size of one archive entry")
	fs.StringVar(&maxArchiveBytes, "max-archive-bytes", maxArchiveBytes, "maximum total uncompressed size of one archive")
	fs.IntVar(&maxEntries, "max-entries", dircompare.DefaultMaxEntries, "maximum files compared across both trees (negative disables the check)")
	fs.BoolVar(&diffExit, "diff-exit-code", false, "exit 1 when differences exist (usage errors exit 2, runtime errors 3)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff dir [flags] LEFT RIGHT

Recursively compare two directory trees, or archives (.zip/.tar/.tar.gz/.tgz),
by file content. Reports files that were added (+), removed (-), or changed (~).
Unchanged files are hidden unless --all is given.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if listFilterSets {
		for _, name := range dircompare.BuiltinFilterSetNames() {
			fmt.Fprintln(stdout, name)
		}
		return exitOK
	}
	set, embedded, err := dircompare.ResolveFilterSets(filterFile, filterSets.values)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	oldPath, newPath := "", ""
	if fs.NArg() == 2 {
		oldPath, newPath = fs.Arg(0), fs.Arg(1)
	} else if fs.NArg() == 0 && embedded != nil && embedded.Old != "" && embedded.New != "" {
		oldPath, newPath = resolveDirProjectPath(filterFile, embedded.Old), resolveDirProjectPath(filterFile, embedded.New)
	} else {
		fmt.Fprintln(stderr, "error: dir needs exactly two paths: LEFT RIGHT (or a directory project with both paths)")
		return exitUsage
	}
	includes.values = append(includes.values, set.Includes...)
	excludes.values = append(excludes.values, set.Excludes...)
	if embedded != nil {
		if compareBy == "" {
			compareBy = embedded.CompareBy
		}
		if embedded.Hidden {
			includeHidden = true
		}
		if embedded.Workers > 0 {
			workers = embedded.Workers
		}
	}
	if set.Expression != "" {
		if strings.TrimSpace(filterExpression) == "" {
			filterExpression = set.Expression
		} else {
			filterExpression = "(" + filterExpression + ") and (" + set.Expression + ")"
		}
	}
	filter, err := dircompare.ParseFilter(filterExpression)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if quick {
		if compareBy != "" && compareBy != "quick" {
			fmt.Fprintln(stderr, "error: --quick cannot be combined with another --compare-by method")
			return exitUsage
		}
		compareBy = "quick"
	}
	method, err := dircompare.ParseCompareMethod(compareBy)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	formats := 0
	for _, selected := range []bool{jsonOut, tsvOut, htmlPath != "", csvPath != ""} {
		if selected {
			formats++
		}
	}
	if formats > 1 {
		fmt.Fprintln(stderr, "error: --json, --tsv, --html, and --csv cannot be combined")
		return exitUsage
	}
	if workers < 1 || workers > 64 {
		fmt.Fprintln(stderr, "error: --workers must be from 1 to 64")
		return exitUsage
	}
	entryLimit, err := parsePositiveByteSize("--max-archive-entry-bytes", maxArchiveEntryBytes)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	totalLimit, err := parsePositiveByteSize("--max-archive-bytes", maxArchiveBytes)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if entryLimit > totalLimit {
		fmt.Fprintln(stderr, "error: --max-archive-entry-bytes cannot exceed --max-archive-bytes")
		return exitUsage
	}
	res, err := dircompare.CompareAny(oldPath, newPath, dircompare.Options{
		Excludes: excludes.values, Includes: includes.values, IncludeHidden: includeHidden,
		Filter: filter, CompareBy: method, Workers: workers, MaxArchiveEntryBytes: entryLimit, MaxArchiveBytes: totalLimit,
		MaxEntries: maxEntries,
	})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}

	if htmlPath != "" {
		title := oldPath + " vs " + newPath
		if err := atomicfile.Write(htmlPath, atomicfile.Options{}, func(w io.Writer) error {
			return dirreport.WriteHTML(w, res, title, all)
		}); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
		writeDirReportSummary(stderr, res, htmlPath)
	} else if csvPath != "" {
		if err := atomicfile.Write(csvPath, atomicfile.Options{}, func(w io.Writer) error {
			return dirreport.WriteCSV(w, res, all)
		}); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
		writeDirReportSummary(stderr, res, csvPath)
	} else if jsonOut {
		if err := writeDirJSON(stdout, res); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
	} else if tsvOut {
		if err := writeDirTSV(stdout, res, all); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
		fmt.Fprintf(stderr, "%d added, %d removed, %d changed, %d same\n", res.Added, res.Removed, res.Changed, res.Same)
	} else {
		if err := writeDirText(stdout, stderr, res, all); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
	}
	if diffExit && res.Added+res.Removed+res.Changed > 0 {
		return exitDiff
	}
	return exitOK
}

func resolveDirProjectPath(projectPath, value string) string {
	if value == "" || filepath.IsAbs(value) || projectPath == "" {
		return value
	}
	return filepath.Clean(filepath.Join(filepath.Dir(projectPath), value))
}

func writeDirReportSummary(stderr io.Writer, res *dircompare.Result, path string) {
	fmt.Fprintf(stderr, "%d added, %d removed, %d changed, %d same\nwrote %s\n",
		res.Added, res.Removed, res.Changed, res.Same, path)
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
