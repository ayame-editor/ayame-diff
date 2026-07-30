package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/engine"
	"github.com/hjosugi/ayame-diff/internal/pathutil"
	"github.com/hjosugi/ayame-diff/internal/threeway"
)

type stringChoices map[string]string

func (c stringChoices) String() string { return "" }
func (c stringChoices) Set(value string) error {
	id, side, ok := strings.Cut(value, "=")
	if !ok || id == "" || (side != "left" && side != "right" && side != "base") {
		return fmt.Errorf("choice must be ID=left|right|base")
	}
	c[id] = side
	return nil
}

func runThreeWayCSV(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff 3way csv", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var base, left, right, output string
	var jsonOut, hasHeader, align, allowConflicts, diffExit bool
	keys, keyIndexes, excludes, excludeIndexes := stringFlags(), intFlags(), stringFlags(), intFlags()
	choices := stringChoices{}
	fs.StringVar(&base, "base", "", "base CSV/TSV")
	fs.StringVar(&left, "left", "", "left-derived CSV/TSV")
	fs.StringVar(&right, "right", "", "right-derived CSV/TSV")
	fs.StringVar(&output, "output", "", "write a reconciled CSV/TSV")
	fs.BoolVar(&jsonOut, "json", false, "emit structured JSON")
	fs.BoolVar(&hasHeader, "header", true, "inputs have a header row")
	fs.BoolVar(&align, "align-columns-by-name", true, "align side columns to the base header")
	fs.Var(&keys, "key", "key header name; repeatable")
	fs.Var(&keyIndexes, "key-index", "zero-based key column; repeatable")
	fs.Var(&excludes, "exclude-key", "use every header except this one as key; repeatable")
	fs.Var(&excludeIndexes, "exclude-key-index", "exclude key index; repeatable")
	fs.Var(choices, "choice", "resolve conflict ID=left|right|base; repeatable")
	fs.BoolVar(&allowConflicts, "allow-conflicts", false, "retain BASE rows for unresolved conflicts")
	fs.BoolVar(&diffExit, "diff-exit-code", false, "exit 1 when changed key groups exist")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "ayame-diff 3way csv [flags] --base BASE --left LEFT --right RIGHT\n\nExplicit key columns are required.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if fs.NArg() != 0 || base == "" || left == "" || right == "" {
		fmt.Fprintln(stderr, "error: --base, --left, and --right are required")
		return exitUsage
	}
	if output != "" {
		for _, input := range []string{base, left, right} {
			if pathutil.Equal(output, input) {
				fmt.Fprintln(stderr, "error: merge output must differ from every input")
				return exitUsage
			}
		}
	}
	workers := min(runtime.NumCPU(), 8)
	cfg := engine.Config{HasHeader: hasHeader, AlignColumnsByName: align, KeyNames: keys.values, KeyIndexes: keyIndexes.values, ExcludeKeyNames: excludes.values, ExcludeKeyIndexes: excludeIndexes.values,
		LeftFormat: "auto", RightFormat: "auto", LeftParser: "auto", RightParser: "auto", Partitions: 8, ParseWorkers: workers, Workers: workers,
		MemoryText: "512MiB", PartitionBufferText: "64KiB", MergeFanIn: 8, MaxRecordText: "256MiB"}
	result, err := threeway.CompareCSV(context.Background(), base, left, right, cfg)
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
		for _, event := range result.Events {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", event.ID, event.Kind, strings.Join(event.Key, "\t"))
		}
	}
	if output != "" {
		unresolved, err := threeway.WriteCSVMerge(base, output, result, choices, allowConflicts)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitError
		}
		fmt.Fprintf(stderr, "merged: %s (conflicts=%d unresolved=%d)\n", output, result.Conflicts, unresolved)
	} else {
		fmt.Fprintf(stderr, "%d key groups, %d conflicts\n", len(result.Events), result.Conflicts)
	}
	if diffExit && len(result.Events) > 0 {
		return exitDiff
	}
	return exitOK
}
