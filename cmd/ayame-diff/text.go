package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hjosugi/ayame-diff/internal/diffout"
	"github.com/hjosugi/ayame-diff/internal/encoding"
	"github.com/hjosugi/ayame-diff/internal/engine"
	"github.com/hjosugi/ayame-diff/internal/htmlreport"
	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/linesort"
	"github.com/hjosugi/ayame-diff/internal/linesrc"
)

// diffFlags are the output/algorithm options shared by the text and sorted
// subcommands.
type diffFlags struct {
	json                           bool
	side                           bool
	summary                        bool
	maxHunks                       int
	maxLines                       uint64
	window                         uint64
	width                          int
	word                           bool
	normal                         bool
	patchFormat                    string
	contextLines                   int
	unifiedContext, contextContext optionalInt
	html                           string
	pre                            string
	encoding                       string
	maxLineBytes                   string
	ignoreCase                     bool
	whitespace                     string
	ignoreAllSpace                 bool
	ignoreSpaceChange              bool
	ignoreEOL                      bool
	ignoreTrailingEOL              bool
	lineFilters                    repeatedFlag[string]
	detectMoves                    bool
	moveMinLines                   uint64
	moveMaxCandidates              int
	syncPoints                     syncFlag
}

type syncFlag []linediff.SyncPoint

func (s *syncFlag) String() string {
	parts := make([]string, len(*s))
	for i, point := range *s {
		parts[i] = fmt.Sprintf("%d:%d", point.Old+1, point.New+1)
	}
	return strings.Join(parts, ",")
}
func (s *syncFlag) Set(text string) error {
	parts := strings.Split(text, ":")
	if len(parts) != 2 {
		return fmt.Errorf("sync point must be LEFT:RIGHT (1-based line numbers)")
	}
	oldLine, oldErr := strconv.ParseUint(parts[0], 10, 64)
	newLine, newErr := strconv.ParseUint(parts[1], 10, 64)
	if oldErr != nil || newErr != nil || oldLine == 0 || newLine == 0 {
		return fmt.Errorf("sync point must contain positive 1-based line numbers")
	}
	*s = append(*s, linediff.SyncPoint{Old: oldLine - 1, New: newLine - 1})
	return nil
}

type optionalInt struct {
	value int
	set   bool
}

func (o *optionalInt) String() string {
	if !o.set {
		return ""
	}
	return strconv.Itoa(o.value)
}
func (o *optionalInt) Set(text string) error {
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		return fmt.Errorf("context line count must be a non-negative integer")
	}
	o.value, o.set = value, true
	return nil
}

func (d *diffFlags) register(fs *flag.FlagSet) {
	fs.BoolVar(&d.json, "json", false, "emit the diff as JSON")
	fs.BoolVar(&d.side, "side-by-side", false, "two-column (left | right) output")
	fs.BoolVar(&d.side, "side", false, "alias for --side-by-side")
	fs.BoolVar(&d.summary, "summary", false, "print only the one-line summary")
	fs.BoolVar(&d.normal, "normal", false, "GNU normal-diff (patch) output")
	fs.StringVar(&d.patchFormat, "format", "", "patch format: normal, context, or unified")
	fs.IntVar(&d.contextLines, "context-lines", 3, "context lines for context/unified patch formats")
	fs.Var(&d.unifiedContext, "U", "unified patch with N context lines")
	fs.Var(&d.contextContext, "C", "context patch with N context lines")
	fs.StringVar(&d.html, "html", "", "write a self-contained HTML report to this file")
	fs.StringVar(&d.pre, "pre", "", "preprocess each input through this shell command before diffing (e.g. --pre 'jq -S .')")
	fs.BoolVar(&d.word, "word", false, "highlight changed words in replace hunks (unified)")
	fs.StringVar(&d.encoding, "encoding", "auto", "input encoding: auto, utf-8, utf-16le, utf-16be, shift_jis, euc-jp, iso-2022-jp")
	fs.StringVar(&d.maxLineBytes, "max-line-bytes", "64MiB", "reject a single line longer than this (0 disables the check)")
	fs.BoolVar(&d.ignoreCase, "ignore-case", false, "ignore case when comparing lines")
	fs.StringVar(&d.whitespace, "ignore-whitespace", "none", "whitespace handling: none, change (collapse runs), all (remove)")
	fs.BoolVar(&d.ignoreAllSpace, "ignore-all-space", false, "ignore all whitespace (GNU diff compatible alias)")
	fs.BoolVar(&d.ignoreSpaceChange, "ignore-space-change", false, "collapse whitespace runs (GNU diff compatible alias)")
	fs.BoolVar(&d.ignoreEOL, "ignore-eol", false, "ignore CRLF/LF line-ending differences")
	fs.BoolVar(&d.ignoreTrailingEOL, "ignore-trailing-eol", false, "ignore only a missing final line ending")
	d.lineFilters = stringFlags()
	fs.Var(&d.lineFilters, "filter-line", "remove regex matches before comparison; repeatable")
	fs.BoolVar(&d.detectMoves, "detect-moves", false, "detect exact delete/insert blocks as moves")
	fs.Uint64Var(&d.moveMinLines, "move-min-lines", 2, "minimum lines in a moved block")
	fs.IntVar(&d.moveMaxCandidates, "move-max-candidates", 10000, "maximum delete and insert candidates examined")
	fs.Var(&d.syncPoints, "sync", "force corresponding lines LEFT:RIGHT (1-based, repeatable)")
	fs.IntVar(&d.maxHunks, "max-hunks", 200, "maximum hunks to print; the rest are still counted")
	fs.Uint64Var(&d.maxLines, "max-lines", 200, "maximum lines shown per hunk side")
	fs.Uint64Var(&d.window, "window", 128, "resync look-ahead window when lines differ")
	fs.IntVar(&d.width, "width", 160, "total width for --side-by-side")
}

// lineLimit resolves --max-line-bytes. A single line is the one thing the
// sliding-window reader cannot bound on its own, so a file with no line breaks
// would otherwise be fully resident (#137). Zero disables the check.
func (d *diffFlags) lineLimit() (int, error) {
	value, err := engine.ParseByteSize(d.maxLineBytes)
	if err != nil {
		return 0, fmt.Errorf("--max-line-bytes: %w", err)
	}
	if value > int64(math.MaxInt) {
		return 0, fmt.Errorf("--max-line-bytes is too large")
	}
	return int(value), nil
}

func (d *diffFlags) format() diffout.Format {
	format, _, _, _ := d.outputFormat()
	return format
}

func (d *diffFlags) outputFormat() (diffout.Format, int, bool, error) {
	patchRequested := d.normal || strings.TrimSpace(d.patchFormat) != "" || d.unifiedContext.set || d.contextContext.set
	if patchRequested && (d.json || d.summary || d.side || d.html != "") {
		return 0, 0, false, fmt.Errorf("patch format cannot be combined with JSON, summary, side-by-side, or HTML output")
	}
	switch {
	case d.json:
		return diffout.JSON, 0, false, nil
	case d.summary:
		return diffout.Summary, 0, false, nil
	case d.side:
		return diffout.SideBySide, 0, false, nil
	}
	selected := strings.ToLower(strings.TrimSpace(d.patchFormat))
	if d.normal {
		if selected != "" && selected != "normal" {
			return 0, 0, false, fmt.Errorf("--normal conflicts with --format=%s", selected)
		}
		selected = "normal"
	}
	if d.unifiedContext.set {
		if selected != "" && selected != "unified" {
			return 0, 0, false, fmt.Errorf("-U conflicts with --format=%s", selected)
		}
		selected = "unified"
	}
	if d.contextContext.set {
		if selected != "" && selected != "context" {
			return 0, 0, false, fmt.Errorf("-C conflicts with --format=%s", selected)
		}
		selected = "context"
	}
	contextLines := d.contextLines
	if d.unifiedContext.set {
		contextLines = d.unifiedContext.value
	}
	if d.contextContext.set {
		contextLines = d.contextContext.value
	}
	if contextLines < 0 {
		return 0, 0, false, fmt.Errorf("patch context lines must be non-negative")
	}
	switch selected {
	case "":
		return diffout.Unified, 0, false, nil
	case "normal":
		return diffout.Normal, 0, true, nil
	case "context":
		return diffout.PatchContext, contextLines, true, nil
	case "unified":
		return diffout.PatchUnified, contextLines, true, nil
	default:
		return 0, 0, false, fmt.Errorf("--format must be normal, context, or unified")
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

func (d diffFlags) comparisonOptions() (linediff.Options, error) {
	whitespace := d.whitespace
	if d.ignoreAllSpace && d.ignoreSpaceChange {
		return linediff.Options{}, fmt.Errorf("--ignore-all-space and --ignore-space-change cannot be combined")
	}
	if d.ignoreAllSpace {
		whitespace = "all"
	}
	if d.ignoreSpaceChange {
		whitespace = "change"
	}
	if whitespace != "none" && whitespace != "change" && whitespace != "all" {
		return linediff.Options{}, fmt.Errorf("--ignore-whitespace must be none, change, or all")
	}
	filters, err := linediff.CompileLineFilters(d.lineFilters.values)
	if err != nil {
		return linediff.Options{}, err
	}
	return linediff.Options{
		IgnoreCase: d.ignoreCase, Whitespace: whitespaceMode(whitespace),
		IgnoreEOL: d.ignoreEOL, IgnoreTrailingEOL: d.ignoreTrailingEOL,
		LineFilters: filters,
	}, nil
}

// emitDiff runs the line diff and writes it in the selected format. Hunks/JSON
// go to stdout, the summary to stderr, matching the CSV mode's split.
func emitDiff(old, new linediff.Lines, d diffFlags, oldLabel, newLabel string, stdout, stderr io.Writer) error {
	format, contextLines, patch, err := d.outputFormat()
	if err != nil {
		return err
	}
	maxHunks := d.maxHunks
	if patch {
		maxHunks = math.MaxInt
	}
	if err := linediff.ValidateSyncPoints(d.syncPoints, old.Count(), new.Count()); err != nil {
		return err
	}
	comparison, err := d.comparisonOptions()
	if err != nil {
		return err
	}
	comparison.MaxHunks, comparison.Window, comparison.SyncPoints = maxHunks, d.window, d.syncPoints
	res, err := linediff.DiffWith(old, new, comparison)
	if err != nil {
		return err
	}
	if d.detectMoves {
		linediff.DetectMoves(old, new, &res, linediff.MoveOptions{
			MinLines: d.moveMinLines, MaxCandidates: d.moveMaxCandidates,
		})
	}
	if d.html != "" {
		f, err := os.Create(d.html)
		if err != nil {
			return err
		}
		if err := htmlreport.Write(f, old, new, res, oldLabel+" vs "+newLabel); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		// Still print the one-line summary to stderr; the HTML is the report.
		if err := diffout.Write(io.Discard, stderr, old, new, res, diffout.Options{Format: diffout.Summary}); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "wrote %s\n", d.html)
		return nil
	}
	opts := diffout.Options{
		Format: format, MaxLines: d.maxLines, Width: d.width, Word: d.word,
		Context: contextLines, ContextSet: patch, OldLabel: oldLabel, NewLabel: newLabel,
		OldTime: fileModTime(oldLabel), NewTime: fileModTime(newLabel),
	}
	return diffout.Write(stdout, stderr, old, new, res, opts)
}

func fileModTime(path string) time.Time {
	if path == "-" {
		return time.Time{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// runText implements: ayame-diff text [flags] LEFT RIGHT
func runText(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff text", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var d diffFlags
	d.register(fs)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff text [flags] LEFT RIGHT

Line-level diff of two text files (plain or .gz), comparing by row order.
Uses a bounded resync window, so it stays linear and memory-bounded on huge
inputs. LEFT or RIGHT may be - for standard input, or clip: for the OS clipboard.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := parseDiffArgs(fs, args); err != nil {
		return reportFlagError(err, stderr)
	}
	if _, _, _, err := d.outputFormat(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	maxLine, err := d.lineLimit()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	oldSrc, closeOld, err := openSource(fs.Arg(0), d.encoding, d.pre, maxLine, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer closeOld()
	newSrc, closeNew, err := openSource(fs.Arg(1), d.encoding, d.pre, maxLine, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer closeNew()
	if err := emitDiff(oldSrc, newSrc, d, fs.Arg(0), fs.Arg(1), stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	return exitOK
}

// runSorted implements: ayame-diff sorted [flags] LEFT RIGHT
//
// Both inputs are sorted line-wise, then compared with the text line diff. This
// finds the set/multiset difference of two files whose row order differs.
//
// v1 sorts in memory; an external, memory-bounded line sort is tracked in #7.
func runSorted(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff sorted", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var d diffFlags
	d.register(fs)
	var numeric, reverse bool
	fs.BoolVar(&numeric, "numeric", false, "sort by leading numeric value")
	fs.BoolVar(&numeric, "n", false, "alias for --numeric")
	fs.BoolVar(&reverse, "reverse", false, "reverse the sort order")
	fs.BoolVar(&reverse, "r", false, "alias for --reverse")
	var sortMemory, tempDir string
	fs.StringVar(&sortMemory, "sort-memory", "256MiB", "line data held in memory before the sort spills to disk")
	fs.StringVar(&tempDir, "temp-dir", "", "parent directory for sort spill files (default: TMPDIR)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff sorted [flags] LEFT RIGHT

Sort both text files (plain or .gz) line-wise, then diff. Use when the two
files hold the same rows in a different order.

Files larger than --sort-memory spill sorted runs to --temp-dir and are merged,
so inputs bigger than RAM still compare. On many systems TMPDIR is RAM-backed;
point --temp-dir at a real disk when sorting very large files.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := parseDiffArgs(fs, args); err != nil {
		return reportFlagError(err, stderr)
	}
	memoryBytes, err := engine.ParseByteSize(sortMemory)
	if err != nil {
		fmt.Fprintln(stderr, "error: --sort-memory:", err)
		return exitUsage
	}
	if memoryBytes <= 0 {
		fmt.Fprintln(stderr, "error: --sort-memory must be greater than zero")
		return exitUsage
	}
	if _, _, patch, err := d.outputFormat(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	} else if patch {
		fmt.Fprintln(stderr, "error: patch formats require text mode; sorted output cannot be applied to the original file")
		return exitUsage
	}
	maxLine, err := d.lineLimit()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	oldSrc, closeOld, err := openSource(fs.Arg(0), d.encoding, d.pre, maxLine, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer closeOld()
	newSrc, closeNew, err := openSource(fs.Arg(1), d.encoding, d.pre, maxLine, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer closeNew()
	// Sort through the spilling sorter rather than materializing both sides:
	// a sorted comparison of files larger than memory must complete, not be
	// OOM-killed (#137).
	opts := linesort.Options{Numeric: numeric, Reverse: reverse, MemoryBytes: memoryBytes, TempDir: tempDir}
	oldLines, err := linesort.SortSource(oldSrc, opts)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer oldLines.Close()
	newLines, err := linesort.SortSource(newSrc, opts)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer newLines.Close()
	if err := emitDiff(oldLines, newLines, d, fs.Arg(0), fs.Arg(1), stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	return exitOK
}

// parseDiffArgs parses fs and validates the two positional LEFT RIGHT paths.
func parseDiffArgs(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("%s needs exactly two paths: LEFT RIGHT", fs.Name())
	}
	return nil
}

func reportFlagError(err error, stderr io.Writer) int {
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	fmt.Fprintln(stderr, "error:", err)
	return exitUsage
}

// maxPipedInputBytes bounds the paths that cannot stream. A file is read
// through linesrc's sliding window, but stdin and a --pre command are pipes:
// their length is unknowable in advance and their content has to be
// materialized before it can be split into lines. Refusing past this point
// keeps the promise that a comparison either completes or explains itself,
// rather than being OOM-killed (#137). A var so tests can shrink it.
var maxPipedInputBytes int64 = 1 << 30 // 1 GiB

// errPipedInputTooLarge names the way forward: a file argument streams within
// bounded memory, so the fix is usually to compare files rather than pipes.
func errPipedInputTooLarge(what string) error {
	return fmt.Errorf("%s is larger than %d bytes, which cannot be compared without holding it all in memory; "+
		"write it to a file and pass that path instead — files are streamed within bounded memory", what, maxPipedInputBytes)
}

// readAllBounded reads r, failing with an actionable error past the cap. It
// reads one byte beyond the limit so hitting it exactly is not an error.
func readAllBounded(r io.Reader, what string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxPipedInputBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxPipedInputBytes {
		return nil, errPipedInputTooLarge(what)
	}
	return data, nil
}

// openSource returns a line source for path plus a close func. A path of "-"
// reads standard input; a non-empty pre command preprocesses the input first.
// Both are pipes and must be materialized, so both are bounded by
// maxPipedInputBytes; a plain file is streamed with bounded memory instead.
func openSource(path, encHint, pre string, maxLine int, stderr io.Writer) (linediff.Lines, func(), error) {
	if pre != "" {
		lines, err := preprocessLines(path, encHint, pre, stderr)
		return lines, func() {}, err
	}
	if path == "-" {
		lines, err := readStdin(encHint)
		return lines, func() {}, err
	}
	if isClipboardPath(path) {
		lines, err := readClipboard(encHint)
		return lines, func() {}, err
	}
	f, err := linesrc.OpenEncodingLimit(path, encHint, maxLine)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

// preprocessLines runs pre as a shell command with path's content on stdin
// (stdin itself when path is "-"), then decodes and splits its output. This is
// the "prediffer/unpacker" hook: normalize or transform inputs before diffing
// (e.g. --pre 'jq -S .' to canonicalize JSON).
func preprocessLines(path, encHint, pre string, stderr io.Writer) (linediff.Lines, error) {
	var stdin io.Reader = os.Stdin
	if isClipboardPath(path) {
		data, err := loadClipboardBytes()
		if err != nil {
			return nil, err
		}
		stdin = bytes.NewReader(data)
	} else if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
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
	cmd.Stderr = stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("preprocess %q on %s: %w", pre, path, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("preprocess %q on %s: %w", pre, path, err)
	}
	// Read through the cap rather than cmd.Output()'s unbounded buffer, so a
	// preprocessor that emits far more than it consumes cannot exhaust memory.
	out, readErr := readAllBounded(pipe, fmt.Sprintf("the output of %q", pre))
	if readErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("preprocess %q on %s: %w", pre, path, readErr)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("preprocess %q on %s: %w", pre, path, err)
	}
	dname := encoding.Detect(out, encHint)
	decoded, err := io.ReadAll(encoding.Decoder(bytes.NewReader(out), dname))
	if err != nil {
		return nil, err
	}
	decoded = bytes.TrimPrefix(decoded, []byte("\xef\xbb\xbf"))
	return linediff.SplitTextLines(string(decoded)), nil
}

// readStdin reads all of stdin, decodes it to UTF-8 (encHint, "auto" to detect),
// strips a UTF-8 BOM, and splits into lines.
func readStdin(encHint string) (linediff.Lines, error) {
	data, err := readAllBounded(os.Stdin, "standard input")
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	name := encoding.Detect(data, encHint)
	decoded, err := io.ReadAll(encoding.Decoder(bytes.NewReader(data), name))
	if err != nil {
		return nil, fmt.Errorf("decoding stdin: %w", err)
	}
	decoded = bytes.TrimPrefix(decoded, []byte("\xef\xbb\xbf"))
	return linediff.SplitTextLines(string(decoded)), nil
}
