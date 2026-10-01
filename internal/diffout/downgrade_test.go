package diffout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

// TestRenderingSkipsDowngradedHunks guards the contract that a dismissed
// whitespace/case-only hunk never reaches applyable or display output (#269).
func TestRenderingSkipsDowngradedHunks(t *testing.T) {
	t.Parallel()
	old := linediff.StringLines{"a", "b", "c"}
	new := linediff.StringLines{"a ", "b", "c!"}
	res := linediff.Result{
		OldLines: 3, NewLines: 3, HunkCount: 1,
		Hunks: []linediff.Hunk{
			{Kind: linediff.Replace, OldStart: 0, OldLen: 1, NewStart: 0, NewLen: 1, Downgraded: true},
			{Kind: linediff.Replace, OldStart: 2, OldLen: 1, NewStart: 2, NewLen: 1},
		},
	}
	for _, format := range []Format{Unified, Normal, SideBySide} {
		var w, summary bytes.Buffer
		if err := Write(&w, &summary, old, new, res, Options{Format: format}); err != nil {
			t.Fatalf("format %v: %v", format, err)
		}
		got := w.String()
		if strings.Contains(got, "a ") {
			t.Fatalf("format %v rendered a dismissed hunk:\n%s", format, got)
		}
		if !strings.Contains(got, "c!") {
			t.Fatalf("format %v dropped the real hunk:\n%s", format, got)
		}
	}
}
