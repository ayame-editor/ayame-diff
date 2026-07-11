package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/hjosugi/ayame-diff/internal/diffout"
	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/linesort"
	"github.com/hjosugi/ayame-diff/internal/linesrc"
)

// diffFlags are the output/algorithm options shared by the text and sorted
// subcommands.
type diffFlags struct {
	json       bool
	side       bool
	summary    bool
	maxHunks   int
	maxLines   uint64
	window     uint64
	width      int
	word       bool
	encoding   string
	ignoreCase bool
	whitespace string
}

func (d *diffFlags) register(fs *flag.FlagSet) {
	fs.BoolVar(&d.json, "json", false, "emit the diff as JSON")
	fs.BoolVar(&d.side, "side-by-side", false, "two-column (old | new) output")
	fs.BoolVar(&d.side, "side", false, "alias for --side-by-side")
	fs.BoolVar(&d.summary, "summary", false, "print only the one-line summary")
	fs.BoolVar(&d.word, "word", false, "highlight changed words in replace hunks (unified)")
	fs.StringVar(&d.encoding, "encoding", "auto", "input encoding: auto, utf-8, utf-16le, utf-16be, shift_jis, euc-jp, iso-2022-jp")
	fs.BoolVar(&d.ignoreCase, "ignore-case", false, "ignore case when comparing lines")
	fs.StringVar(&d.whitespace, "ignore-whitespace", "none", "whitespace handling: none, change (collapse runs), all (remove)")
	fs.IntVar(&d.maxHunks, "max-hunks", 200, "maximum hunks to print; the rest are still counted")
	fs.Uint64Var(&d.maxLines, "max-lines", 200, "maximum lines shown per hunk side")
	fs.Uint64Var(&d.window, "window", 128, "resync look-ahead window when lines differ")
	fs.IntVar(&d.width, "width", 160, "total width for --side-by-side")
}

func (d *diffFlags) format() diffout.Format {
	switch {
	case d.json:
		return diffout.JSON
	case d.summary:
		return diffout.Summary
	case d.side:
		return diffout.SideBySide
	default:
		return diffout.Unified
	}
}

// whitespaceMode maps the --ignore-whitespace flag to the linediff enum.
func whitespaceMode(s string) linediff.Whitespace {
	switch s {
	case "change":
		return linediff.WSChange
	case "all":
		return linediff.WSAll
	default:
		return linediff.WSKeep
	}
}

// emitDiff runs the line diff and writes it in the selected format. Hunks/JSON
// go to stdout, the summary to stderr, matching the CSV mode's split.
func emitDiff(old, new linediff.Lines, d diffFlags) {
	res := linediff.DiffWith(old, new, linediff.Options{
		MaxHunks:   d.maxHunks,
		Window:     d.window,
		IgnoreCase: d.ignoreCase,
		Whitespace: whitespaceMode(d.whitespace),
	})
	opts := diffout.Options{Format: d.format(), MaxLines: d.maxLines, Width: d.width, Word: d.word}
	if err := diffout.Write(os.Stdout, os.Stderr, old, new, res, opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

// runText implements: ayame-diff text [flags] OLD NEW
func runText(args []string) {
	fs := flag.NewFlagSet("ayame-diff text", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var d diffFlags
	d.register(fs)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff text [flags] OLD NEW

Line-level diff of two text files (plain or .gz), comparing by row order.
Uses a bounded resync window, so it stays linear and memory-bounded on huge
inputs.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if !parseDiffArgs(fs, args) {
		return
	}

	oldSrc := openLines(fs.Arg(0), d.encoding)
	defer oldSrc.Close()
	newSrc := openLines(fs.Arg(1), d.encoding)
	defer newSrc.Close()
	emitDiff(oldSrc, newSrc, d)
}

// runSorted implements: ayame-diff sorted [flags] OLD NEW
//
// Both inputs are sorted line-wise, then compared with the text line diff. This
// finds the set/multiset difference of two files whose row order differs.
//
// v1 sorts in memory; an external, memory-bounded line sort is tracked in #7.
func runSorted(args []string) {
	fs := flag.NewFlagSet("ayame-diff sorted", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var d diffFlags
	d.register(fs)
	var numeric, reverse bool
	fs.BoolVar(&numeric, "numeric", false, "sort by leading numeric value")
	fs.BoolVar(&numeric, "n", false, "alias for --numeric")
	fs.BoolVar(&reverse, "reverse", false, "reverse the sort order")
	fs.BoolVar(&reverse, "r", false, "alias for --reverse")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff sorted [flags] OLD NEW

Sort both text files (plain or .gz) line-wise, then diff. Use when the two
files hold the same rows in a different order.

Note: v1 sorts in memory.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if !parseDiffArgs(fs, args) {
		return
	}

	oldLines, err := linesort.Sorted(fs.Arg(0), numeric, reverse, d.encoding)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	newLines, err := linesort.Sorted(fs.Arg(1), numeric, reverse, d.encoding)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	emitDiff(oldLines, newLines, d)
}

// parseDiffArgs parses fs and validates the two positional OLD NEW paths.
// It returns false when the caller should return (help was requested).
func parseDiffArgs(fs *flag.FlagSet, args []string) bool {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return false
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if fs.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "error: %s needs exactly two paths: OLD NEW\n", fs.Name())
		os.Exit(2)
	}
	return true
}

func openLines(path, encHint string) *linesrc.FileLines {
	f, err := linesrc.OpenEncoding(path, encHint)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	return f
}
