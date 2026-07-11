// Package linesort reads a text file's lines (plain or gzip, via linesrc) and
// returns them sorted, for the `sorted` comparison used by both the CLI and the
// local server. v1 sorts in memory; a memory-bounded external sort is tracked
// in hjosugi/ayame-diff#7.
package linesort

import (
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
// when either side is not a number (mirroring `sort -n`).
func numericLess(a, b string) bool {
	fa, ea := strconv.ParseFloat(strings.TrimSpace(a), 64)
	fb, eb := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if ea == nil && eb == nil {
		if fa != fb {
			return fa < fb
		}
		return a < b
	}
	return a < b
}
