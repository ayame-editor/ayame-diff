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

func TestDisplayWidthUnicodeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "emoji", text: "🙂", want: 2},
		{name: "emoji modifier", text: "👍🏽", want: 2},
		{name: "emoji ZWJ family", text: "👨‍👩‍👧‍👦", want: 2},
		{name: "regional flag", text: "🇯🇵", want: 2},
		{name: "keycap", text: "1️⃣", want: 2},
		{name: "combining mark", text: "e\u0301", want: 1},
		{name: "controls", text: "a\x00\x1fb", want: 2},
		{name: "Hangul Jamo", text: "각", want: 2},
		{name: "Hangul Jamo Extended-A", text: "ꥠ", want: 2},
		{name: "Kana Supplement", text: "𛀀", want: 2},
		// U+1F6D8 is Neutral and not Emoji_Presentation. The old broad
		// U+1F300..U+1FAFF check incorrectly counted every such rune as wide.
		{name: "neutral supplementary symbol", text: "\U0001f6d8", want: 1},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DisplayWidth(tt.text); got != tt.want {
				t.Fatalf("DisplayWidth(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestEastAsianAmbiguousWidthOption(t *testing.T) {
	t.Parallel()
	const ambiguous = "○※α"
	if got := DisplayWidth(ambiguous); got != 3 {
		t.Fatalf("default ambiguous width = %d, want 3", got)
	}
	opts := Options{EastAsianAmbiguousWide: true}
	if got := DisplayWidthWithOptions(ambiguous, opts); got != 6 {
		t.Fatalf("CJK ambiguous width = %d, want 6", got)
	}
	if got := PadRightWithOptions("○", 4, opts); got != "○  " {
		t.Fatalf("PadRightWithOptions = %q, want %q", got, "○  ")
	}
}

func TestTruncateKeepsEmojiClusterIntact(t *testing.T) {
	t.Parallel()
	const family = "👨‍👩‍👧‍👦"
	if got := Truncate(family+"abcd", 5); got != family+"..." {
		t.Fatalf("Truncate = %q, want intact family plus ellipsis", got)
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
