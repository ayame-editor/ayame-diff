// Package linesort reads a text file's lines (plain or gzip, via linesrc) and
// returns them sorted, for the `sorted` comparison used by both the CLI and the
// local server. v1 sorts in memory; a memory-bounded external sort is tracked
// in hjosugi/ayame-diff#7.
package linesort

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/hjosugi/ayame-diff/internal/linediff"
	"github.com/hjosugi/ayame-diff/internal/linesrc"
)

// Sorted reads every line of path (decoded from encHint, "auto" to detect) and
// returns them sorted. When numeric, lines are ordered by their parsed leading
// numeric value (falling back to lexical); when reverse, the order is inverted.
func Sorted(path string, numeric, reverse bool, encHint string) (linediff.StringLines, error) {
	src, err := linesrc.OpenEncoding(path, encHint)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	n := src.Count()
	lines := make([]string, 0, n)
	for i := uint64(0); i < n; i++ {
		s, _ := src.Line(i)
		lines = append(lines, s)
	}

	return SortLines(lines, numeric, reverse), nil
}

// SortLines sorts an in-memory slice of lines (used for the sorted mode over
// already-read sources such as stdin).
func SortLines(lines []string, numeric, reverse bool) linediff.StringLines {
	out := append([]string(nil), lines...)
	less := lexLess
	if numeric {
		less = numericLess
	}
	sort.SliceStable(out, func(a, b int) bool {
		if reverse {
			return less(out[b], out[a])
		}
		return less(out[a], out[b])
	})
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
