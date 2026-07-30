package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	"github.com/hjosugi/ayame-diff/internal/engine"
	"github.com/hjosugi/ayame-diff/internal/project"
)

var version = "dev"

// Exit codes form a stable taxonomy so scripts can tell "differences found"
// apart from "something failed" (#113). Runtime failures use 3 (not 1) so exit
// 1 is reserved for a real diff under --diff-exit-code.
//
//	0    success — completed; no differences reported
//	1    differences found (only when --diff-exit-code is set)
//	2    usage error — bad flags, arguments, or incompatible options
//	3    runtime error — I/O, comparison, server, or update failure
//	130  interrupted (Ctrl-C / SIGTERM)
const (
	exitOK        = 0
	exitDiff      = 1
	exitUsage     = 2
	exitError     = 3
	exitInterrupt = 130
)

const rootUsage = `ayame-diff compares text, binary files, directories, and huge CSV/TSV data.

Usage:
  ayame-diff <command> [options]
  ayame-diff LEFT RIGHT               compare two text files

Examples:
  ayame-diff gui a.txt b.txt          open a comparison in the browser
  ayame-diff text a.txt b.txt         print a text diff
  ayame-diff dir old-dir new-dir      compare directory trees

Subcommands:
  csv             CSV/TSV key comparison
  text            line diff of two text files
  sorted          sort both files, then line-diff
  dir             compare directories or archives
  bin             byte-level binary/hex diff
  3way            compare BASE, LEFT, and RIGHT (text or CSV)
  serve           run the local web UI
  gui             run the web UI and open it in a browser
  update          self-update to the latest release
  remove          uninstall a standalone installation
  shell-install   register file-manager integration
  shell-uninstall remove file-manager integration
  shell-select    handle a file-manager selection

Run 'ayame-diff <command> --help' for command-specific options.
CSV flags without an explicit command remain supported for compatibility.

Exit codes:
  0    success (no differences)
  1    differences found (with --diff-exit-code)
  2    usage error (bad flags or arguments)
  3    runtime error (I/O, comparison, server, or update failure)
  130  interrupted
`

const csvUsage = `ayame-diff csv compares huge CSV/TSV files whose row order differs.

This (csv) mode requires:
  --left PATH                 Left input file
  --right PATH                Right input file
  --out PATH                  Diff output TSV (use .gz for gzip)

Key selection:
  --key NAME                  Key header name; repeat for multiple columns
  --key-index N               Key column index; repeat for multiple columns
  --exclude-key NAME          Exclude a header from the default all-column key
  --exclude-key-index N       Exclude a column index from the default all-column key

With no key option, every column is used as the key. Include and exclude key
options cannot be mixed. Key indexes are 0-based by default.
Rows with the same selected key but different full content produce two output
rows: one for the left row and one for the right row.`

type subcommandRunner func(args []string, stdout, stderr io.Writer) int

// subcommandRunners is the authoritative list of explicitly dispatched
// commands. Documentation tests use the same registry so a new command cannot
// silently disappear from the top-level help or bilingual command overviews.
var subcommandRunners = map[string]subcommandRunner{
	"csv":             runCSV,
	"text":            runText,
	"sorted":          runSorted,
	"dir":             runDir,
	"bin":             runBin,
	"3way":            runThreeWay,
	"serve":           runServe,
	"gui":             runGUI,
	"update":          runUpdate,
	"remove":          runRemove,
	"shell-install":   runShellInstall,
	"shell-uninstall": runShellUninstall,
	"shell-select":    runShellSelect,
}

// cliOptions contains process-level behavior that does not belong to the diff
// engine's reusable Config API.
type cliOptions struct {
	Engine       engine.Config
	SummaryJSON  string
	DiffExitCode bool
	ShowVersion  bool
	JSON         bool
	Project      string
	SaveProject  string
}

type repeatedFlag[T any] struct {
	values []T
	parse  func(string) (T, error)
	format func(T) string
}

func (r *repeatedFlag[T]) String() string {
	parts := make([]string, len(r.values))
	for i, v := range r.values {
		parts[i] = r.format(v)
	}
	return strings.Join(parts, ",")
}
func (r *repeatedFlag[T]) Set(text string) error {
	v, err := r.parse(text)
	if err != nil {
		return err
	}
	r.values = append(r.values, v)
	return nil
}

func stringFlags() repeatedFlag[string] {
	return repeatedFlag[string]{
		parse: func(value string) (string, error) {
			if value == "" {
				return "", errors.New("value must not be empty")
			}
			return value, nil
		},
		format: func(value string) string { return value },
	}
}

func intFlags() repeatedFlag[int] {
	return repeatedFlag[int]{
		parse: func(value string) (int, error) {
			n, err := strconv.Atoi(value)
			if err != nil {
				return 0, fmt.Errorf("invalid integer %q: %w", value, err)
			}
			return n, nil
		},
		format: strconv.Itoa,
	}
}

type optionalFloat struct {
	value float64
	set   bool
}

func (o *optionalFloat) String() string {
	if !o.set {
		return ""
	}
	return strconv.FormatFloat(o.value, 'g', -1, 64)
}
func (o *optionalFloat) Set(text string) error {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("tolerance must be a finite non-negative number")
	}
	o.value, o.set = value, true
	return nil
}

type columnToleranceFlag struct {
	values  []engine.ColumnTolerance
	byIndex bool
}

func (f *columnToleranceFlag) String() string { return "" }
func (f *columnToleranceFlag) Set(text string) error {
	selector, valueText, ok := strings.Cut(text, "=")
	if !ok || strings.TrimSpace(selector) == "" {
		return fmt.Errorf("column tolerance must be COLUMN=VALUE")
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("column tolerance must be a finite non-negative number")
	}
	tolerance := engine.ColumnTolerance{Value: value, ByIndex: f.byIndex}
	if f.byIndex {
		index, err := strconv.Atoi(selector)
		if err != nil {
			return fmt.Errorf("invalid tolerance column index %q", selector)
		}
		tolerance.Index = index
	} else {
		tolerance.Name = selector
	}
	f.values = append(f.values, tolerance)
	return nil
}

var runEngine = engine.Run

func main() {
	os.Exit(runGuarded(os.Args[1:], os.Stdout, os.Stderr))
}

// runGuarded converts a panic anywhere in the command into the documented
// runtime-failure exit code with a message a user can act on (#137). Without
// it the Go runtime prints a raw stack trace and exits 2 — which collides with
// exitUsage, so a script could not tell a crash from a bad flag. The stack
// still goes to stderr, because a crash the user cannot report is worse than a
// noisy one.
func runGuarded(args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		value := recover()
		if value == nil {
			return
		}
		fmt.Fprintf(stderr, "ayame-diff: internal error: %v\n", value)
		fmt.Fprintln(stderr, "This is a bug. Please report it with the command you ran and the trace below:")
		fmt.Fprintf(stderr, "%s\n", debug.Stack())
		code = exitError
	}()
	return run(args, stdout, stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	// Migration aid: the interactive TUI was removed (#37). Give former
	// --interactive users a clear pointer instead of a bare flag-parse error.
	for _, a := range args {
		if a == "--interactive" || a == "-interactive" {
			fmt.Fprintln(stderr, "ayame-diff: the interactive setup UI was removed.")
			fmt.Fprintln(stderr, "Pass --left, --right, and --out (plus any key options) directly. See --help.")
			return exitUsage
		}
	}
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		printVersion(stdout)
		return exitOK
	}
	if len(args) == 0 || (len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")) {
		fmt.Fprint(stdout, rootUsage)
		return exitOK
	}
	if paths, gui, ok := quickLaunchArgs(args); ok {
		if gui {
			return runGUI(paths, stdout, stderr)
		}
		left, leftErr := os.Stat(paths[0])
		right, rightErr := os.Stat(paths[1])
		if leftErr == nil && rightErr == nil && left.IsDir() && right.IsDir() {
			return runDir(paths, stdout, stderr)
		}
		return runText(paths, stdout, stderr)
	}

	// Subcommand dispatch (ADR 0002). A bare invocation (or one that starts
	// with a flag) stays on the CSV/TSV key comparison for backward
	// compatibility; new line-oriented modes live under explicit subcommands.
	if command := subcommand(args); command != "" {
		return subcommandRunners[command](args[1:], stdout, stderr)
	}
	return runCSV(args, stdout, stderr)
}

// quickLaunchArgs recognizes the file-manager-friendly forms A B and
// --gui A B (also A B --gui). Other flags retain the legacy CSV behavior.
func quickLaunchArgs(args []string) (paths []string, gui, ok bool) {
	if len(args) > 0 && subcommand(args) != "" {
		return nil, false, false
	}
	positional := false
	for _, arg := range args {
		switch {
		case !positional && arg == "--gui":
			gui = true
		case !positional && arg == "--":
			positional = true
		case !positional && strings.HasPrefix(arg, "-"):
			return nil, false, false
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) == 2 || (gui && len(paths) == 1) {
		return paths, gui, true
	}
	return nil, false, false
}

// subcommand returns args[0] when it names a known subcommand, else "".
func subcommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if _, ok := subcommandRunners[args[0]]; ok {
		return args[0]
	}
	return ""
}

func printVersion(stdout io.Writer) {
	fmt.Fprintf(stdout, "ayame-diff %s (%s/%s, %s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// runCSV is the CSV/TSV key-comparison mode (the original behavior).
func runCSV(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, flagOutput(args, stdout, stderr))
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if opts.ShowVersion {
		printVersion(stdout)
		return exitOK
	}
	cfg := opts.Engine
	if opts.Project != "" {
		loaded, loadErr := project.Load(opts.Project)
		if loadErr != nil {
			fmt.Fprintln(stderr, "error:", loadErr)
			return exitError
		}
		cfg = loaded.CSV
	}
	cfg.Log = stderr

	if len(args) == 0 {
		// A package manager may execute a newly installed portable command
		// without arguments to verify that its alias starts successfully. Treat
		// that probe like --help instead of reporting missing CSV operands.
		fmt.Fprintln(stdout, csvUsage)
		return exitOK
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if opts.SaveProject != "" {
		if err := cfg.Validate(); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUsage
		}
		if err := project.Save(opts.SaveProject, project.Project{Mode: "csv", CSV: cfg, Report: project.Report{CellDiff: cfg.CellDiff, OutputFormat: cfg.OutputFormat}}); err != nil {
			fmt.Fprintln(stderr, "error: save project:", err)
			return exitError
		}
		fmt.Fprintln(stderr, "saved project:", opts.SaveProject)
	}
	summary, err := runEngine(ctx, cfg)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		if errors.Is(err, context.Canceled) {
			return exitInterrupt
		}
		return exitError
	}
	if opts.SummaryJSON != "" {
		data, marshalErr := json.MarshalIndent(summary, "", "  ")
		if marshalErr != nil {
			fmt.Fprintln(stderr, "error: encode summary:", marshalErr)
			return exitError
		}
		if writeErr := os.WriteFile(opts.SummaryJSON, append(data, '\n'), 0o644); writeErr != nil {
			fmt.Fprintln(stderr, "error: write summary:", writeErr)
			return exitError
		}
	}

	fmt.Fprintf(stderr,
		"done: left=%d right=%d equal=%d diff_rows=%d left_only=%d right_only=%d changed_left=%d changed_right=%d elapsed=%s\n",
		summary.LeftRows, summary.RightRows, summary.EqualRows, summary.DiffRows,
		summary.LeftOnly, summary.RightOnly, summary.ChangedLeft, summary.ChangedRight, summary.Elapsed)
	if len(summary.ColumnChanges) > 0 {
		fmt.Fprint(stderr, "changed columns:")
		for i, column := range summary.ColumnChanges {
			if i == 10 {
				fmt.Fprint(stderr, " ...")
				break
			}
			fmt.Fprintf(stderr, " %s=%d", column.Name, column.Count)
		}
		fmt.Fprintln(stderr)
	}
	if opts.DiffExitCode && summary.DiffRows > 0 {
		return exitDiff
	}
	return exitOK
}

func parseFlags(args []string, output ...io.Writer) (cliOptions, error) {
	var opts cliOptions
	cfg := &opts.Engine
	keys, keyIndexes := stringFlags(), intFlags()
	excludeKeys, excludeKeyIndexes := stringFlags(), intFlags()
	ignoreColumns, ignoreColumnIndexes, lineFilters := stringFlags(), intFlags(), stringFlags()
	var tolerance optionalFloat
	tolerances := columnToleranceFlag{}
	toleranceIndexes := columnToleranceFlag{byIndex: true}
	var ignoreAllSpace, ignoreSpaceChange bool
	fs := flag.NewFlagSet("ayame-diff", flag.ContinueOnError)
	if len(output) > 0 {
		fs.SetOutput(output[0])
	} else {
		fs.SetOutput(io.Discard)
	}
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), csvUsage)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}

	registerIOFlags(fs, cfg)
	registerKeyFlags(fs, cfg, &keys, &keyIndexes, &excludeKeys, &excludeKeyIndexes)
	registerParseFlags(fs, cfg)
	fs.BoolVar(&cfg.IgnoreCase, "ignore-case", false, "ignore case in keys and values")
	fs.StringVar(&cfg.IgnoreWhitespace, "ignore-whitespace", "none", "whitespace handling: none, change, or all")
	fs.BoolVar(&ignoreAllSpace, "ignore-all-space", false, "ignore all whitespace")
	fs.BoolVar(&ignoreSpaceChange, "ignore-space-change", false, "collapse whitespace runs")
	fs.BoolVar(&cfg.IgnoreEOL, "ignore-eol", false, "accepted for parity; CSV parsing already ignores CRLF/LF differences")
	fs.BoolVar(&cfg.IgnoreTrailingEOL, "ignore-trailing-eol", false, "accepted for parity; CSV records ignore trailing EOL")
	fs.Var(&lineFilters, "filter-line", "remove regex matches from fields before comparison; repeatable")
	fs.Var(&ignoreColumns, "ignore-column", "header excluded from key (by default) and value comparison; repeatable")
	fs.Var(&ignoreColumnIndexes, "ignore-column-index", "column index excluded from key (by default) and value comparison; repeatable")
	fs.Var(&tolerance, "tolerance", "absolute numeric tolerance for compared value columns")
	fs.Var(&tolerances, "column-tolerance", "per-header numeric tolerance NAME=VALUE; repeatable")
	fs.Var(&toleranceIndexes, "column-tolerance-index", "per-index numeric tolerance INDEX=VALUE; repeatable")
	fs.BoolVar(&cfg.CellDiff, "cell-diff", false, "add _changed_cols and per-column change counts")
	fs.BoolVar(&opts.JSON, "json", false, "write structured cell differences as JSON Lines to --out")
	fs.StringVar(&cfg.OutputFormat, "output-format", "tsv", "output format: tsv or jsonl")
	registerPerfFlags(fs, cfg)
	fs.BoolVar(&cfg.Progress, "progress", true, "print periodic progress to stderr")
	fs.StringVar(&opts.SummaryJSON, "summary-json", "", "write machine-readable summary JSON")
	fs.BoolVar(&opts.DiffExitCode, "diff-exit-code", false, "exit 1 when differences exist (usage errors exit 2, runtime errors 3)")
	fs.BoolVar(&cfg.OutputHeader, "output-header", true, "write a header to the output TSV")
	fs.BoolVar(&opts.ShowVersion, "version", false, "print version and exit")
	fs.StringVar(&opts.Project, "project", "", "load a versioned .ayamediff.json project")
	fs.StringVar(&opts.SaveProject, "save-project", "", "save the effective CSV configuration as a project")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg.KeyNames = append([]string(nil), keys.values...)
	cfg.KeyIndexes = append([]int(nil), keyIndexes.values...)
	cfg.ExcludeKeyNames = append([]string(nil), excludeKeys.values...)
	cfg.ExcludeKeyIndexes = append([]int(nil), excludeKeyIndexes.values...)
	if ignoreAllSpace && ignoreSpaceChange {
		return opts, fmt.Errorf("--ignore-all-space and --ignore-space-change cannot be combined")
	}
	if ignoreAllSpace {
		cfg.IgnoreWhitespace = "all"
	}
	if ignoreSpaceChange {
		cfg.IgnoreWhitespace = "change"
	}
	cfg.LineFilters = append([]string(nil), lineFilters.values...)
	cfg.IgnoreColumnNames = append([]string(nil), ignoreColumns.values...)
	cfg.IgnoreColumnIndexes = append([]int(nil), ignoreColumnIndexes.values...)
	cfg.Tolerance, cfg.ToleranceSet = tolerance.value, tolerance.set
	cfg.ColumnTolerances = append(append([]engine.ColumnTolerance(nil), tolerances.values...), toleranceIndexes.values...)
	if opts.JSON {
		cfg.OutputFormat, cfg.CellDiff = "jsonl", true
	}
	return opts, nil
}

func registerIOFlags(fs *flag.FlagSet, cfg *engine.Config) {
	fs.StringVar(&cfg.LeftPath, "left", "", "left/old CSV or TSV path")
	fs.StringVar(&cfg.RightPath, "right", "", "right/new CSV or TSV path")
	fs.StringVar(&cfg.OutputPath, "out", "", "diff TSV path; .gz enables gzip")
}

func registerKeyFlags(fs *flag.FlagSet, cfg *engine.Config, keys *repeatedFlag[string], keyIndexes *repeatedFlag[int], excludeKeys *repeatedFlag[string], excludeKeyIndexes *repeatedFlag[int]) {
	fs.Var(keys, "key", "key header name; repeatable")
	fs.Var(keyIndexes, "key-index", "key column index; repeatable")
	fs.Var(excludeKeys, "exclude-key", "header name excluded from the all-column key; repeatable")
	fs.Var(excludeKeyIndexes, "exclude-key-index", "column index excluded from the all-column key; repeatable")
	fs.IntVar(&cfg.IndexBase, "index-base", 0, "base for key index options: 0 or 1")
	fs.BoolVar(&cfg.HasHeader, "header", true, "treat the first record as a header")
	fs.BoolVar(&cfg.AlignColumnsByName, "align-columns-by-name", true, "align right columns to left header order")
}

func registerParseFlags(fs *flag.FlagSet, cfg *engine.Config) {
	fs.StringVar(&cfg.LeftFormat, "left-format", "auto", "auto, csv, or tsv")
	fs.StringVar(&cfg.RightFormat, "right-format", "auto", "auto, csv, or tsv")
	fs.StringVar(&cfg.LeftDelimiter, "left-delimiter", "", "override delimiter: comma, tab, \\t, or one ASCII character")
	fs.StringVar(&cfg.RightDelimiter, "right-delimiter", "", "override delimiter: comma, tab, \\t, or one ASCII character")
	fs.StringVar(&cfg.LeftParser, "left-parser", "auto", "auto, simple, or rfc4180")
	fs.StringVar(&cfg.RightParser, "right-parser", "auto", "auto, simple, or rfc4180")
	fs.BoolVar(&cfg.LazyQuotes, "lazy-quotes", false, "allow malformed quotes in RFC 4180 parser")
	fs.BoolVar(&cfg.TrimLeadingSpace, "trim-leading-space", false, "trim leading spaces in RFC 4180 fields")
}

func registerPerfFlags(fs *flag.FlagSet, cfg *engine.Config) {
	fs.IntVar(&cfg.Partitions, "partitions", 256, fmt.Sprintf("hash partition count; power of two, %d..%d", engine.MinPartitions, engine.MaxPartitions))
	fs.IntVar(&cfg.ParseWorkers, "parse-workers", min(runtime.NumCPU(), 8), "parallel readers for uncompressed simple parser")
	fs.IntVar(&cfg.Workers, "workers", min(runtime.NumCPU(), 8), "parallel partition comparison workers")
	fs.StringVar(&cfg.MemoryText, "memory", "2GiB", "total sorting memory, e.g. 512MiB or 8GiB")
	fs.StringVar(&cfg.PartitionBufferText, "partition-buffer", "256KiB", "buffer per partition file")
	fs.IntVar(&cfg.MergeFanIn, "merge-fan-in", 32, fmt.Sprintf("maximum sorted runs merged at once (%d..%d)", engine.MinMergeFanIn, engine.MaxMergeFanIn))
	fs.StringVar(&cfg.MaxRecordText, "max-record-bytes", "256MiB", "maximum encoded key plus row size")
	fs.StringVar(&cfg.TempDir, "temp-dir", "", "parent directory for temporary work data")
	fs.StringVar(&cfg.WorkDir, "work-dir", "", "exact empty work directory")
	fs.BoolVar(&cfg.KeepTemp, "keep-temp", false, "keep partition and sort files")
}

func flagOutput(args []string, stdout, stderr io.Writer) io.Writer {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return stdout
		}
	}
	return stderr
}
