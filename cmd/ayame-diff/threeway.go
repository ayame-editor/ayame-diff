package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/linesrc"
	"github.com/hjosugi/ayame-diff/internal/threeway"
)

type conflictChoices map[int]string

func (c conflictChoices) String() string { return "" }
func (c conflictChoices) Set(value string) error {
	idText, side, ok := strings.Cut(value, "=")
	id, err := strconv.Atoi(idText)
	if !ok || err != nil || id < 0 || (side != "left" && side != "right" && side != "base") {
		return fmt.Errorf("choice must be EVENT=left|right|base")
	}
	c[id] = side
	return nil
}

func runThreeWay(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "csv" {
		return runThreeWayCSV(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "text" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("ayame-diff 3way text", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var jsonOut, allowConflicts, diffExit bool
	var output, enc, whitespace, format string
	var window uint64
	var ignoreCase bool
	filters := stringFlags()
	choices := conflictChoices{}
	fs.BoolVar(&jsonOut, "json", false, "emit structured three-way JSON")
	fs.StringVar(&format, "format", "unified", "output format: unified or json")
	fs.StringVar(&output, "output", "", "write an auto-merged file (conflicts require --choice or --allow-conflicts)")
	fs.StringVar(&enc, "encoding", "auto", "input encoding")
	fs.Uint64Var(&window, "window", 128, "two-way resync look-ahead")
	fs.BoolVar(&ignoreCase, "ignore-case", false, "ignore case for comparison")
	fs.StringVar(&whitespace, "ignore-whitespace", "none", "none, change, or all")
	fs.Var(&filters, "filter-line", "remove regex matches before comparison; repeatable")
	fs.Var(choices, "choice", "resolve conflict EVENT=left|right|base; repeatable")
	fs.BoolVar(&allowConflicts, "allow-conflicts", false, "write standard conflict markers for unresolved conflicts")
	fs.BoolVar(&diffExit, "diff-exit-code", false, "exit 1 when changes or conflicts exist")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff 3way [text] [flags] BASE LEFT RIGHT
ayame-diff 3way csv [flags] --base BASE --left LEFT --right RIGHT

Compare two derived text files against a common base. Independent and
identical edits merge automatically; overlapping different edits are conflicts.`)
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
	if fs.NArg() != 3 {
		fmt.Fprintln(stderr, "error: 3way text needs BASE LEFT RIGHT")
		return exitUsage
	}
	if format == "json" {
		jsonOut = true
	} else if format != "unified" {
		fmt.Fprintln(stderr, "error: --format must be unified or json")
		return exitUsage
	}
	if output != "" {
		outAbs, _ := filepath.Abs(output)
		for _, input := range fs.Args() {
			inputAbs, _ := filepath.Abs(input)
			if outAbs == inputAbs {
				fmt.Fprintln(stderr, "error: merge output must differ from every input")
				return exitUsage
			}
		}
	}
	base, err := linesrc.OpenEncoding(fs.Arg(0), enc)
	if err != nil {
		fmt.Fprintln(stderr, "error: base:", err)
		return exitError
	}
	defer base.Close()
	left, err := linesrc.OpenEncoding(fs.Arg(1), enc)
	if err != nil {
		fmt.Fprintln(stderr, "error: left:", err)
		return exitError
	}
	defer left.Close()
	right, err := linesrc.OpenEncoding(fs.Arg(2), enc)
	if err != nil {
		fmt.Fprintln(stderr, "error: right:", err)
		return exitError
	}
	defer right.Close()
	compiled, err := linediff.CompileLineFilters(filters.values)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if whitespace != "none" && whitespace != "change" && whitespace != "all" {
		fmt.Fprintln(stderr, "error: --ignore-whitespace must be none, change, or all")
		return exitUsage
	}
	result, err := threeway.Compare(base, left, right, linediff.Options{Window: window, IgnoreCase: ignoreCase, Whitespace: whitespaceMode(whitespace), LineFilters: compiled})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	if jsonOut {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
	} else {
		writeThreeWayText(stdout, result)
	}
	if output != "" {
		// Capture the base file's encoding/BOM/EOL before MergeLines streams it,
		// so the written merge round-trips them instead of BOM-less UTF-8/LF (#159).
		profile := threeway.ProfileOf(base)
		lines, unresolved, err := threeway.MergeLines(base, result, choices, allowConflicts)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
		if err := threeway.WriteMerged(output, lines, profile); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
		fmt.Fprintf(stderr, "merged: %s (conflicts=%d unresolved=%d)\n", output, result.Conflicts, unresolved)
	} else {
		fmt.Fprintf(stderr, "%d events, %d conflicts\n", len(result.Events), result.Conflicts)
	}
	if diffExit && len(result.Events) > 0 {
		return exitDiff
	}
	return exitOK
}

func writeThreeWayText(w io.Writer, result threeway.Result) {
	for _, event := range result.Events {
		fmt.Fprintf(w, "@@ BASE %d,%d %s #%d @@\n", event.BaseStart+1, event.BaseLen, event.Kind, event.ID)
		if event.Kind == threeway.Conflict {
			fmt.Fprintln(w, "<<<<<<< LEFT")
			for _, line := range event.Left {
				fmt.Fprintln(w, line)
			}
			fmt.Fprintln(w, "||||||| BASE")
			for _, line := range event.Base {
				fmt.Fprintln(w, line)
			}
			fmt.Fprintln(w, "=======")
			for _, line := range event.Right {
				fmt.Fprintln(w, line)
			}
			fmt.Fprintln(w, ">>>>>>> RIGHT")
			continue
		}
		fmt.Fprintln(w, "--- BASE")
		for _, line := range event.Base {
			fmt.Fprintln(w, "-"+line)
		}
		fmt.Fprintln(w, "+++ RESULT")
		selected := event.Left
		if event.Kind == threeway.RightOnly {
			selected = event.Right
		}
		for _, line := range selected {
			fmt.Fprintln(w, "+"+line)
		}
	}
}
