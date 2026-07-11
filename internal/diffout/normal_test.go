package diffout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

func TestNormalFormatMatchesGNUDiff(t *testing.T) {
	t.Parallel()
	old := linediff.SplitLines("a\nb\nc\nd\n")
	new := linediff.SplitLines("a\nX\nc\nd\ne\n")
	res := linediff.Diff(old, new, 200, 128)

	var w, sum bytes.Buffer
	if err := Write(&w, &sum, old, new, res, Options{Format: Normal}); err != nil {
		t.Fatal(err)
	}
	// Exactly what `diff old new` prints for this input.
	want := "2c2\n< b\n---\n> X\n4a5\n> e\n"
	if w.String() != want {
		t.Fatalf("normal output:\n%q\nwant:\n%q", w.String(), want)
	}
}

func TestNormalDeleteAndRange(t *testing.T) {
	t.Parallel()
	// A pure deletion of two lines -> "Nd" with "< " lines and a range.
	old := linediff.SplitLines("a\nb\nc\nd\n")
	new := linediff.SplitLines("a\nd\n")
	res := linediff.Diff(old, new, 200, 128)
	var w, sum bytes.Buffer
	if err := Write(&w, &sum, old, new, res, Options{Format: Normal}); err != nil {
		t.Fatal(err)
	}
	got := w.String()
	if !strings.Contains(got, "d1") || !strings.Contains(got, "< b") || !strings.Contains(got, "< c") {
		t.Fatalf("delete normal output:\n%s", got)
	}
}
