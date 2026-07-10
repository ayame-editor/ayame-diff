package textwidth

import (
	"strings"
	"testing"
)

func TestDisplayWidthJapanese(t *testing.T) {
	t.Parallel()
	if got := DisplayWidth("A東京"); got != 5 { // A(1) + 東(2) + 京(2)
		t.Fatalf("DisplayWidth = %d, want 5", got)
	}
	// base "e" (width 1) + combining acute U+0301 (width 0)
	combining := string([]rune{'e', 0x0301})
	if got := DisplayWidth(combining); got != 1 {
		t.Fatalf("combining DisplayWidth = %d, want 1", got)
	}
}

func TestInlineEscapesControls(t *testing.T) {
	t.Parallel()
	got := Inline("name\tline\nnext\x01")
	// No raw control characters survive, so the result stays on one row.
	for _, r := range got {
		if r < 0x20 {
			t.Fatalf("Inline left control char %U in %q", r, got)
		}
	}
	// Tab and newline are escaped to their two-character forms.
	if !strings.Contains(got, `\t`) || !strings.Contains(got, `\n`) {
		t.Fatalf("Inline did not escape tab/newline: %q", got)
	}
}

func TestTruncateAndPad(t *testing.T) {
	t.Parallel()
	if got := Truncate("東京都", 4); DisplayWidth(got) > 4 {
		t.Fatalf("Truncate width = %d, want <= 4 (%q)", DisplayWidth(got), got)
	}
	if got := PadRight("A東", 6); DisplayWidth(got) != 6 { // A(1) + 東(2) + 3 spaces
		t.Fatalf("PadRight width = %d, want 6 (%q)", DisplayWidth(got), got)
	}
}
