package linediff

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// TestReplaceHunksAreAlwaysOneToOne pins an invariant the GUI's unified view
// depends on (#115). That view stacks a hunk's cells in DOM order, which reads
// as a patch — removal above addition — only because a changed hunk holds a
// single pair. Were Replace ever to cover several lines at once, unified would
// render -a +a -b +b where a patch has -a -b +a +b, and nothing else in the
// codebase would notice.
//
// The property holds by construction: the resync fallback emits Replace with
// OldLen and NewLen hard-coded to 1, so consecutive replaced lines become
// consecutive 1:1 hunks. This test exists so that changing that — coalescing
// them, say, as an optimization — fails here rather than silently in the
// browser.
func TestReplaceHunksAreAlwaysOneToOne(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, label string, res Result) {
		t.Helper()
		for i, h := range res.Hunks {
			if h.Kind != Replace {
				continue
			}
			if h.OldLen != 1 || h.NewLen != 1 {
				t.Errorf("%s: hunk %d is Replace with OldLen=%d NewLen=%d; "+
					"the unified view assumes 1:1 and would render a changed block out of patch order",
					label, i, h.OldLen, h.NewLen)
			}
		}
	}

	// Shapes that most plausibly coalesce: runs of consecutive changed lines,
	// and replaced blocks of unequal size.
	cases := []struct {
		name     string
		old, new string
	}{
		{"single change", "a\nb\nc\n", "a\nB\nc\n"},
		{"two consecutive changes", "a\nb\nc\nd\n", "a\nB\nC\nd\n"},
		{"four consecutive changes", "h\na\nb\nc\nd\nf\n", "h\nA\nB\nC\nD\nf\n"},
		{"whole file replaced", "a\nb\nc\n", "x\ny\nz\n"},
		{"shrinking block", "h\na\nb\nc\nd\nf\n", "h\nX\nY\nf\n"},
		{"growing block", "h\na\nb\nf\n", "h\nW\nX\nY\nZ\nf\n"},
		{"change then insert", "a\nb\nc\n", "a\nB\nc\nd\ne\n"},
		{"interleaved changes", "a\nb\nc\nd\ne\n", "A\nb\nC\nd\nE\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			check(t, c.name, diffStrings(t, c.old, c.new, 1000, 128))
		})
	}

	// Random inputs, since the fixtures above only cover shapes I thought of.
	t.Run("randomised", func(t *testing.T) {
		t.Parallel()
		rng := rand.New(rand.NewSource(20260719))
		for i := 0; i < 300; i++ {
			var oldLines, newLines []string
			for line := 0; line < 1+rng.Intn(30); line++ {
				oldLines = append(oldLines, fmt.Sprintf("line-%d", rng.Intn(8)))
			}
			for line := 0; line < 1+rng.Intn(30); line++ {
				newLines = append(newLines, fmt.Sprintf("line-%d", rng.Intn(8)))
			}
			old, neu := strings.Join(oldLines, "\n")+"\n", strings.Join(newLines, "\n")+"\n"
			check(t, fmt.Sprintf("random %d: %q -> %q", i, old, neu),
				diffStrings(t, old, neu, 1000, 128))
		}
	})
}
