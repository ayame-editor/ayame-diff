// Package linesort reads a text file's lines (plain or gzip, via linesrc) and
// returns them sorted, for the `sorted` comparison used by both the CLI and the
// local server.
//
// Sorting is memory-bounded: input that fits the budget is sorted in memory,
// and anything larger spills sorted runs to disk and merges them with a bounded
// fan-in, so a file far bigger than RAM sorts instead of being OOM-killed
// (#7, #137). Unlike the other line sources, which stream, a sort must see
// every line before it can emit the first one — that is why this is the one
// path that needs spill rather than a window.
package linesort

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ayame-editor/ayame-diff/internal/encoding"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/linesrc"
)

// DefaultMemoryBytes bounds the line data held in memory before the sort
// spills. It is a var so callers and tests can lower it; the default is chosen
// to keep ordinary files on the fast in-memory path.
var DefaultMemoryBytes int64 = 256 << 20 // 256 MiB

// perLineOverhead approximates the string header and slice slot that accompany
// each retained line, so the budget reflects real footprint rather than raw
// byte length alone.
const perLineOverhead = 48

// Options configures a sort.
type Options struct {
	// Numeric orders by parsed leading numeric value instead of lexically.
	Numeric bool
	// Reverse inverts the order.
	Reverse bool
	// MemoryBytes bounds resident line data; 0 selects DefaultMemoryBytes.
	MemoryBytes int64
	// TempDir hosts spill files; "" uses the OS temporary directory.
	TempDir string
}

// Result is a sorted view of the input lines. Close releases the spill files;
// it is safe to call (and a no-op) when the sort stayed in memory.
type Result struct {
	linediff.Lines
	cleanup func() error
}

// Close releases any temporary files the sort created.
func (r *Result) Close() error {
	if r == nil || r.cleanup == nil {
		return nil
	}
	cleanup := r.cleanup
	r.cleanup = nil
	return cleanup()
}

// Spilled reports whether the sort had to go to disk. Used by tests and by
// callers that want to explain a slow comparison.
func (r *Result) Spilled() bool { return r != nil && r.cleanup != nil }

// Sorted reads every line of path (decoded from encHint, "auto" to detect) and
// returns them sorted within the default memory budget. The caller must Close
// the result to release any spill files.
func Sorted(path string, numeric, reverse bool, encHint string) (*Result, error) {
	return SortedWithOptions(path, encHint, Options{Numeric: numeric, Reverse: reverse})
}

// SortedWithOptions is Sorted with an explicit memory budget and spill location.
func SortedWithOptions(path, encHint string, opts Options) (*Result, error) {
	src, err := linesrc.OpenEncoding(path, encHint)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return SortSource(src, opts)
}

// SortSource returns src's lines in sorted order while holding at most
// opts.MemoryBytes of line data. Lines accumulate into a chunk; each time the
// chunk reaches the budget it is sorted and spilled as a run, and the runs are
// merged at the end. An input that never reaches the budget never touches disk.
//
// The ordering is a total order on distinct lines (equal magnitudes and
// non-numeric text both fall back to a lexical tiebreak), so identical lines
// are indistinguishable and the merge needs no stability tiebreak to match the
// in-memory result.
func SortSource(src linediff.Lines, opts Options) (result *Result, resultErr error) {
	budget := opts.MemoryBytes
	if budget <= 0 {
		budget = DefaultMemoryBytes
	}
	less := lessFunc(opts.Numeric, opts.Reverse)

	var (
		chunk      []string
		chunkBytes int64
		runs       []string
		dir        string
	)
	defer func() {
		if resultErr != nil && dir != "" {
			_ = os.RemoveAll(dir)
		}
	}()
	spill := func() error {
		if len(chunk) == 0 {
			return nil
		}
		if dir == "" {
			created, err := os.MkdirTemp(opts.TempDir, "ayame-sort-")
			if err != nil {
				return spillError(opts.TempDir, err)
			}
			dir = created
		}
		path, err := writeRun(dir, len(runs), chunk, less)
		if err != nil {
			return spillError(dir, err)
		}
		runs = append(runs, path)
		// Zero the slots before reusing the array so the spilled lines become
		// collectable instead of staying pinned by the backing array.
		clear(chunk)
		chunk, chunkBytes = chunk[:0], 0
		return nil
	}

	count := src.Count()
	for i := uint64(0); i < count; i++ {
		line, ok := src.Line(i)
		if !ok {
			break
		}
		chunk = append(chunk, line)
		chunkBytes += int64(len(line)) + perLineOverhead
		if chunkBytes >= budget {
			if err := spill(); err != nil {
				return nil, err
			}
		}
	}
	if len(runs) == 0 {
		// Everything fit: keep the fast path, no temporary files.
		sortChunk(chunk, less)
		return &Result{Lines: linediff.StringLines(chunk)}, nil
	}
	if err := spill(); err != nil {
		return nil, err
	}
	merged, err := mergeRuns(dir, runs, less)
	if err != nil {
		return nil, spillError(dir, err)
	}
	// The merged run is the UTF-8, LF-terminated file this package just wrote,
	// so it is read back with a window rather than materialized.
	lines, err := linesrc.OpenEncoding(merged, encoding.UTF8)
	if err != nil {
		return nil, err
	}
	spillDir := dir
	return &Result{Lines: lines, cleanup: func() error {
		closeErr := lines.Close()
		if err := os.RemoveAll(spillDir); err != nil {
			return err
		}
		return closeErr
	}}, nil
}

// spillError explains a failed spill in terms the user can act on. The default
// temporary directory is a RAM-backed tmpfs on many Linux systems, so a sort
// large enough to spill can exhaust it while the disk sits empty — a bare
// "no space left on device" would send the user looking in the wrong place.
func spillError(dir string, err error) error {
	where := dir
	if where == "" {
		where = os.TempDir()
	}
	return fmt.Errorf("the sorted comparison needed temporary space in %s and could not get it: %w\n"+
		"Point it at a filesystem with room using --temp-dir (or the TMPDIR environment variable); "+
		"note that %s is RAM-backed on many systems", where, err, os.TempDir())
}

func sortChunk(lines []string, less func(a, b string) bool) {
	sort.SliceStable(lines, func(a, b int) bool { return less(lines[a], lines[b]) })
}

func lessFunc(numeric, reverse bool) func(a, b string) bool {
	base := lexLess
	if numeric {
		base = numericLess
	}
	if reverse {
		return func(a, b string) bool { return base(b, a) }
	}
	return base
}

// SortLines sorts an already-materialized slice of lines. It is for sources
// that are in memory by nature and separately bounded — pasted scratch text,
// which rides inside the capped JSON body — so it deliberately has no spill
// path. File-backed inputs should use SortSource, which does.
func SortLines(lines []string, numeric, reverse bool) linediff.StringLines {
	out := append([]string(nil), lines...)
	sortChunk(out, lessFunc(numeric, reverse))
	return linediff.StringLines(out)
}

func lexLess(a, b string) bool { return a < b }

// numericLess compares by parsed numeric value, falling back to lexical order
// when a side is not a real number (mirroring `sort -n`). It is a strict-weak
// ordering even with NaN or ±Inf inputs (#165): real numbers order by value,
// with a lexical tiebreak for equal magnitudes; everything else (unparseable
// text and NaN) orders lexically and sorts after the real numbers. Because NaN
// is never treated as a number, the old `fa != fb` / `fa < fb` contradiction —
// which made less(a,b) and less(b,a) both false and broke the sort — cannot
// occur.
func numericLess(a, b string) bool {
	fa, aNum := sortNumber(a)
	fb, bNum := sortNumber(b)
	if aNum && bNum {
		if fa != fb {
			return fa < fb
		}
		return a < b // equal magnitude: deterministic lexical tiebreak
	}
	if aNum != bNum {
		return aNum // real numbers sort before non-numeric text (and NaN)
	}
	return a < b // both non-numeric: lexical
}

// sortNumber reports whether s is an orderable real number and returns its
// value. Overflow/underflow (e.g. 1e400 -> +Inf) still count as numbers so they
// order by magnitude; NaN does not, so it can never violate the ordering.
func sortNumber(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	if math.IsNaN(f) {
		return 0, false
	}
	return f, true
}
