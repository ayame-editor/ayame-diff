package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hjosugi/ayame-diff/internal/engine"
)

var version = "dev"

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	if v == "" {
		return errors.New("value must not be empty")
	}
	*s = append(*s, v)
	return nil
}

type intList []int

func (s *intList) String() string {
	parts := make([]string, len(*s))
	for i, v := range *s {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}
func (s *intList) Set(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid integer %q: %w", v, err)
	}
	*s = append(*s, n)
	return nil
}

func main() {
	args := os.Args[1:]
	// Migration aid: the interactive TUI was removed (#37). Give former
	// --interactive users a clear pointer instead of a bare flag-parse error.
	for _, a := range args {
		if a == "--interactive" || a == "-interactive" {
			fmt.Fprintln(os.Stderr, "ayame-diff: the interactive setup UI was removed.")
			fmt.Fprintln(os.Stderr, "Pass --left, --right, and --out (plus any key options) directly. See --help.")
			os.Exit(2)
		}
	}
	cfg, showVersion, err := parseFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if showVersion {
		fmt.Printf("ayame-diff %s (%s/%s, %s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "ayame-diff: no arguments given; --left, --right, and --out are required.")
		fmt.Fprintln(os.Stderr, "Run 'ayame-diff --help' for usage.")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	started := time.Now()
	summary, err := engine.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	summary.Elapsed = time.Since(started).Round(time.Millisecond).String()

	if cfg.SummaryJSON != "" {
		data, marshalErr := json.MarshalIndent(summary, "", "  ")
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "error: encode summary:", marshalErr)
			os.Exit(2)
		}
		if writeErr := os.WriteFile(cfg.SummaryJSON, append(data, '\n'), 0o644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "error: write summary:", writeErr)
			os.Exit(2)
		}
	}

	fmt.Fprintf(os.Stderr,
		"done: left=%d right=%d equal=%d diff_rows=%d left_only=%d right_only=%d changed_left=%d changed_right=%d elapsed=%s\n",
		summary.LeftRows, summary.RightRows, summary.EqualRows, summary.DiffRows,
		summary.LeftOnly, summary.RightOnly, summary.ChangedLeft, summary.ChangedRight, summary.Elapsed)
	if cfg.DiffExitCode && summary.DiffRows > 0 {
		os.Exit(1)
	}
}

func parseFlags(args []string) (engine.Config, bool, error) {
	var cfg engine.Config
	var keys stringList
	var keyIndexes intList
	var excludeKeys stringList
	var excludeKeyIndexes intList
	var showVersion bool
	fs := flag.NewFlagSet("ayame-diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff compares huge CSV/TSV files whose row order differs.

Required:
  --left PATH                 Left/old input file
  --right PATH                Right/new input file
  --out PATH                  Diff output TSV (use .gz for gzip)

Key selection:
  --key NAME                  Key header name; repeat for multiple columns
  --key-index N               Key column index; repeat for multiple columns
  --exclude-key NAME          Exclude a header from the default all-column key
  --exclude-key-index N       Exclude a column index from the default all-column key

With no key option, every column is used as the key. Include and exclude key
options cannot be mixed. Key indexes are 0-based by default.
Rows with the same selected key but different full content produce two output
rows: one for the left row and one for the right row.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}

	fs.StringVar(&cfg.LeftPath, "left", "", "left/old CSV or TSV path")
	fs.StringVar(&cfg.RightPath, "right", "", "right/new CSV or TSV path")
	fs.StringVar(&cfg.OutputPath, "out", "", "diff TSV path; .gz enables gzip")
	fs.Var(&keys, "key", "key header name; repeatable")
	fs.Var(&keyIndexes, "key-index", "key column index; repeatable")
	fs.Var(&excludeKeys, "exclude-key", "header name excluded from the all-column key; repeatable")
	fs.Var(&excludeKeyIndexes, "exclude-key-index", "column index excluded from the all-column key; repeatable")
	fs.IntVar(&cfg.IndexBase, "index-base", 0, "base for key index options: 0 or 1")
	fs.BoolVar(&cfg.HasHeader, "header", true, "treat the first record as a header")
	fs.BoolVar(&cfg.AlignColumnsByName, "align-columns-by-name", true, "align right columns to left header order")
	fs.StringVar(&cfg.LeftFormat, "left-format", "auto", "auto, csv, or tsv")
	fs.StringVar(&cfg.RightFormat, "right-format", "auto", "auto, csv, or tsv")
	fs.StringVar(&cfg.LeftDelimiter, "left-delimiter", "", "override delimiter: comma, tab, \\t, or one ASCII character")
	fs.StringVar(&cfg.RightDelimiter, "right-delimiter", "", "override delimiter: comma, tab, \\t, or one ASCII character")
	fs.StringVar(&cfg.LeftParser, "left-parser", "auto", "auto, simple, or rfc4180")
	fs.StringVar(&cfg.RightParser, "right-parser", "auto", "auto, simple, or rfc4180")
	fs.BoolVar(&cfg.LazyQuotes, "lazy-quotes", false, "allow malformed quotes in RFC 4180 parser")
	fs.BoolVar(&cfg.TrimLeadingSpace, "trim-leading-space", false, "trim leading spaces in RFC 4180 fields")
	fs.IntVar(&cfg.Partitions, "partitions", 256, "hash partition count; power of two, 2..1024")
	fs.IntVar(&cfg.ParseWorkers, "parse-workers", min(runtime.NumCPU(), 8), "parallel readers for uncompressed simple parser")
	fs.IntVar(&cfg.Workers, "workers", min(runtime.NumCPU(), 8), "parallel partition comparison workers")
	fs.StringVar(&cfg.MemoryText, "memory", "2GiB", "total sorting memory, e.g. 512MiB or 8GiB")
	fs.StringVar(&cfg.PartitionBufferText, "partition-buffer", "256KiB", "buffer per partition file")
	fs.IntVar(&cfg.MergeFanIn, "merge-fan-in", 32, "maximum sorted runs merged at once")
	fs.StringVar(&cfg.MaxRecordText, "max-record-bytes", "256MiB", "maximum encoded key plus row size")
	fs.StringVar(&cfg.TempDir, "temp-dir", "", "parent directory for temporary work data")
	fs.StringVar(&cfg.WorkDir, "work-dir", "", "exact empty work directory")
	fs.BoolVar(&cfg.KeepTemp, "keep-temp", false, "keep partition and sort files")
	fs.BoolVar(&cfg.Progress, "progress", true, "print periodic progress to stderr")
	fs.StringVar(&cfg.SummaryJSON, "summary-json", "", "write machine-readable summary JSON")
	fs.BoolVar(&cfg.DiffExitCode, "diff-exit-code", false, "exit 1 when differences exist; errors exit 2")
	fs.BoolVar(&cfg.OutputHeader, "output-header", true, "write a header to the output TSV")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return cfg, false, err
	}
	if fs.NArg() != 0 {
		return cfg, false, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg.KeyNames = append([]string(nil), keys...)
	cfg.KeyIndexes = append([]int(nil), keyIndexes...)
	cfg.ExcludeKeyNames = append([]string(nil), excludeKeys...)
	cfg.ExcludeKeyIndexes = append([]int(nil), excludeKeyIndexes...)
	return cfg, showVersion, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
