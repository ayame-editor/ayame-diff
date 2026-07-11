package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/hjosugi/ayame-diff/internal/dircompare"
)

// runDir implements: ayame-diff dir [flags] OLD_DIR NEW_DIR
func runDir(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff dir", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var all, jsonOut bool
	excludes := stringFlags()
	fs.BoolVar(&all, "all", false, "include unchanged (same) files in the output")
	fs.BoolVar(&jsonOut, "json", false, "emit the result as JSON")
	fs.Var(&excludes, "exclude", "glob to skip (repeatable), matched on the relative path or base name")
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

	res, err := dircompare.CompareAny(fs.Arg(0), fs.Arg(1), dircompare.Options{Excludes: excludes.values})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	if jsonOut {
		if err := writeDirJSON(stdout, res); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
	} else {
		if err := writeDirText(stdout, stderr, res, all); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
	}
	return 0
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
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldSize int64  `json:"old_size"`
	NewSize int64  `json:"new_size"`
}

func writeDirJSON(stdout io.Writer, res *dircompare.Result) error {
	entries := make([]dirEntryJSON, len(res.Entries))
	for i, e := range res.Entries {
		entries[i] = dirEntryJSON{Path: e.Path, Status: e.Status.String(), OldSize: e.OldSize, NewSize: e.NewSize}
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
