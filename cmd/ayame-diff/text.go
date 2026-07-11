package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/hjosugi/ayame-diff/internal/diffout"
	"github.com/hjosugi/ayame-diff/internal/encoding"
	"github.com/hjosugi/ayame-diff/internal/htmlreport"
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
	normal     bool
	html       string
	pre        string
	encoding   string
	ignoreCase bool
	whitespace string
}

func (d *diffFlags) register(fs *flag.FlagSet) {
	fs.BoolVar(&d.json, "json", false, "emit the diff as JSON")
	fs.BoolVar(&d.side, "side-by-side", false, "two-column (old | new) output")
	fs.BoolVar(&d.side, "side", false, "alias for --side-by-side")
	fs.BoolVar(&d.summary, "summary", false, "print only the one-line summary")
	fs.BoolVar(&d.normal, "normal", false, "GNU normal-diff (patch) output")
	fs.StringVar(&d.html, "html", "", "write a self-contained HTML report to this file")
	fs.StringVar(&d.pre, "pre", "", "preprocess each input through this shell command before diffing (e.g. --pre 'jq -S .')")
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
	case d.normal:
		return diffout.Normal
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
func emitDiff(old, new linediff.Lines, d diffFlags, title string) {
	res := linediff.DiffWith(old, new, linediff.Options{
		MaxHunks:   d.maxHunks,
		Window:     d.window,
		IgnoreCase: d.ignoreCase,
		Whitespace: whitespaceMode(d.whitespace),
	})
	if d.html != "" {
		f, err := os.Create(d.html)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		if err := htmlreport.Write(f, old, new, res, title); err != nil {
			f.Close()
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		f.Close()
		// Still print the one-line summary to stderr; the HTML is the report.
		_ = diffout.Write(io.Discard, os.Stderr, old, new, res, diffout.Options{Format: diffout.Summary})
		fmt.Fprintf(os.Stderr, "wrote %s\n", d.html)
		return
	}
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
inputs. OLD or NEW may be - to read standard input.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if !parseDiffArgs(fs, args) {
		return
	}

	oldSrc, closeOld := openSource(fs.Arg(0), d.encoding, d.pre)
	defer closeOld()
	newSrc, closeNew := openSource(fs.Arg(1), d.encoding, d.pre)
	defer closeNew()
	emitDiff(oldSrc, newSrc, d, fs.Arg(0)+" vs "+fs.Arg(1))
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

	oldSrc, closeOld := openSource(fs.Arg(0), d.encoding, d.pre)
	defer closeOld()
	newSrc, closeNew := openSource(fs.Arg(1), d.encoding, d.pre)
	defer closeNew()
	oldLines := linesort.SortLines(collectLines(oldSrc), numeric, reverse)
	newLines := linesort.SortLines(collectLines(newSrc), numeric, reverse)
	emitDiff(oldLines, newLines, d, fs.Arg(0)+" vs "+fs.Arg(1)+" (sorted)")
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

// openSource returns a line source for path plus a close func. A path of "-"
// reads standard input; a non-empty pre command preprocesses the input first
// (both read fully into memory). Otherwise a file is streamed with bounded
// memory.
func openSource(path, encHint, pre string) (linediff.Lines, func()) {
	if pre != "" {
		return preprocessLines(path, encHint, pre), func() {}
	}
	if path == "-" {
		return readStdin(encHint), func() {}
	}
	f, err := linesrc.OpenEncoding(path, encHint)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	return f, func() { f.Close() }
}

// preprocessLines runs pre as a shell command with path's content on stdin
// (stdin itself when path is "-"), then decodes and splits its output. This is
// the "prediffer/unpacker" hook: normalize or transform inputs before diffing
// (e.g. --pre 'jq -S .' to canonicalize JSON).
func preprocessLines(path, encHint, pre string) linediff.StringLines {
	var stdin io.Reader = os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		defer f.Close()
		stdin = f
	}
	name, args := "sh", []string{"-c", pre}
	if runtime.GOOS == "windows" {
		name, args = "cmd", []string{"/c", pre}
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: preprocess %q on %s: %v\n", pre, path, err)
		os.Exit(2)
	}
	dname := encoding.Detect(out, encHint)
	decoded, err := io.ReadAll(encoding.Decoder(bytes.NewReader(out), dname))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	decoded = bytes.TrimPrefix(decoded, []byte("\xef\xbb\xbf"))
	return linediff.SplitLines(string(decoded))
}

// readStdin reads all of stdin, decodes it to UTF-8 (encHint, "auto" to detect),
// strips a UTF-8 BOM, and splits into lines.
func readStdin(encHint string) linediff.StringLines {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: reading stdin:", err)
		os.Exit(2)
	}
	name := encoding.Detect(data, encHint)
	decoded, err := io.ReadAll(encoding.Decoder(bytes.NewReader(data), name))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: decoding stdin:", err)
		os.Exit(2)
	}
	decoded = bytes.TrimPrefix(decoded, []byte("\xef\xbb\xbf"))
	return linediff.SplitLines(string(decoded))
}

// collectLines reads every line of l into a slice (for in-memory sorting).
func collectLines(l linediff.Lines) []string {
	n := l.Count()
	out := make([]string, 0, n)
	for i := uint64(0); i < n; i++ {
		s, _ := l.Line(i)
		out = append(out, s)
	}
	return out
}
