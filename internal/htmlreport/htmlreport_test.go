package htmlreport

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

func TestWrite(t *testing.T) {
	t.Parallel()
	old := linediff.SplitLines("the quick brown fox\nsecond\n")
	new := linediff.SplitLines("the slow brown fox\nsecond\nthird\n")
	res := linediff.Diff(old, new, 200, 128)

	var buf bytes.Buffer
	if err := Write(&buf, old, new, res, "a vs b"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"<!doctype html>", "<style>", "a vs b", "class=\"hunk\"",
		"w-del", "w-add", "quick", "slow", "third",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HTML report missing %q", want)
		}
	}
	// balanced enough to be a full document
	if !strings.Contains(out, "</body></html>") {
		t.Fatal("report not closed")
	}
}

func TestWriteEscapesHTML(t *testing.T) {
	t.Parallel()
	old := linediff.SplitLines("<script>alert(1)</script>\n")
	new := linediff.SplitLines("<b>bold</b>\n")
	res := linediff.Diff(old, new, 200, 128)
	var buf bytes.Buffer
	if err := Write(&buf, old, new, res, "x"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Raw markup from the files must never appear un-escaped.
	if strings.Contains(out, "<script>") || strings.Contains(out, "alert(1)</script>") {
		t.Fatalf("raw file content must be HTML-escaped:\n%s", out)
	}
	// The escaped form of "<" must be present (word-diff spans may split the
	// token, so we check the character, not a contiguous "&lt;script&gt;").
	if !strings.Contains(out, "&lt;") {
		t.Fatal("expected escaped '<' in the output")
	}
}
