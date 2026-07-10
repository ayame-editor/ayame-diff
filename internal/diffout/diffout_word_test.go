package diffout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

func TestUnifiedWordMarkers(t *testing.T) {
	t.Parallel()
	old := linediff.StringLines{"the quick brown fox"}
	new := linediff.StringLines{"the slow brown dog"}
	res := linediff.Diff(old, new, 200, 128)
	if res.HunkCount != 1 || res.Hunks[0].Kind != linediff.Replace {
		t.Fatalf("expected one replace hunk, got %+v", res)
	}

	var w, sum bytes.Buffer
	if err := Write(&w, &sum, old, new, res, Options{Format: Unified, Word: true}); err != nil {
		t.Fatal(err)
	}
	got := w.String()
	// Changed words wrapped, unchanged words left plain.
	for _, want := range []string{"[-quick-]", "[-fox-]", "{+slow+}", "{+dog+}"} {
		if !strings.Contains(got, want) {
			t.Fatalf("word output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[-brown-]") || strings.Contains(got, "{+brown+}") {
		t.Fatalf("unchanged word 'brown' should not be marked:\n%s", got)
	}
	if strings.Contains(got, "[-the-]") {
		t.Fatalf("unchanged word 'the' should not be marked:\n%s", got)
	}
}

func TestUnifiedWordDisabledIsPlain(t *testing.T) {
	t.Parallel()
	old := linediff.StringLines{"alpha beta"}
	new := linediff.StringLines{"alpha gamma"}
	res := linediff.Diff(old, new, 200, 128)

	var w, sum bytes.Buffer
	if err := Write(&w, &sum, old, new, res, Options{Format: Unified}); err != nil {
		t.Fatal(err)
	}
	got := w.String()
	if strings.Contains(got, "[-") || strings.Contains(got, "{+") {
		t.Fatalf("word markers must not appear without Options.Word:\n%s", got)
	}
	if !strings.Contains(got, "-alpha beta") || !strings.Contains(got, "+alpha gamma") {
		t.Fatalf("plain unified lines missing:\n%s", got)
	}
}
