package worddiff

import (
	"strings"
	"testing"
)

// These cases mirror ayame-editor's inlineWordDiff behavior so the Go port
// keeps parity with the reference implementation.

// joinSegments concatenates segment text; used to assert that the diff output
// reconstructs the original input on each side.
func joinSegments(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

func segTuples(segs []Segment) []Segment {
	// Return a copy so equality checks compare value slices directly.
	out := make([]Segment, len(segs))
	copy(out, segs)
	return out
}

func equalSegs(a, b []Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTokenize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single word", "hello", []string{"hello"}},
		{
			// Each boundary class is its own token: note the comma and the
			// space after it split apart because whitespace and "other" are
			// separate classes.
			name: "mixed",
			in:   "foo  bar_baz, qux!",
			want: []string{"foo", "  ", "bar_baz", ",", " ", "qux", "!"},
		},
		{
			// Underscore and digits count as word characters.
			name: "word chars",
			in:   "a1_b2",
			want: []string{"a1_b2"},
		},
		{
			// CJK is written without spaces, so each character is its own token
			// (Han + Hiragana here), split from the surrounding punctuation. This
			// lets an inline diff align on individual characters. (#161)
			name: "japanese",
			in:   "日本語です。",
			want: []string{"日", "本", "語", "で", "す", "。"},
		},
		{
			// A base letter keeps its combining mark ("e" + U+0301) as one token
			// instead of stranding the mark. (#161)
			name: "combining mark",
			in:   "café",
			want: []string{"café"},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Tokenize(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("Tokenize(%q) = %#v, want %#v", c.in, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("Tokenize(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		old     string
		new     string
		wantOK  bool
		wantOld []Segment
		wantNew []Segment
	}{
		{
			name:   "identical returns false",
			old:    "the quick brown fox",
			new:    "the quick brown fox",
			wantOK: false,
		},
		{
			// One word swapped mid-sentence: shared prefix and suffix stay
			// unchanged; only the differing word is Changed on each side.
			name:   "single changed word",
			old:    "the quick brown fox",
			new:    "the quick red fox",
			wantOK: true,
			wantOld: []Segment{
				{Text: "the quick ", Changed: false},
				{Text: "brown", Changed: true},
				{Text: " fox", Changed: false},
			},
			wantNew: []Segment{
				{Text: "the quick ", Changed: false},
				{Text: "red", Changed: true},
				{Text: " fox", Changed: false},
			},
		},
		{
			// Pure insertion: everything old is common, new gains a Changed run.
			name:   "pure insertion",
			old:    "hello world",
			new:    "hello big world",
			wantOK: true,
			wantOld: []Segment{
				{Text: "hello world", Changed: false},
			},
			wantNew: []Segment{
				{Text: "hello ", Changed: false},
				{Text: "big ", Changed: true},
				{Text: "world", Changed: false},
			},
		},
		{
			// Pure deletion: mirror of insertion; only Old has a Changed run.
			name:   "pure deletion",
			old:    "hello big world",
			new:    "hello world",
			wantOK: true,
			wantOld: []Segment{
				{Text: "hello ", Changed: false},
				{Text: "big ", Changed: true},
				{Text: "world", Changed: false},
			},
			wantNew: []Segment{
				{Text: "hello world", Changed: false},
			},
		},
		{
			// Trailing append: the new token has no common suffix after it.
			name:   "append at end",
			old:    "foo",
			new:    "foo bar",
			wantOK: true,
			wantOld: []Segment{
				{Text: "foo", Changed: false},
			},
			wantNew: []Segment{
				{Text: "foo", Changed: false},
				{Text: " bar", Changed: true},
			},
		},
		{
			// Complete replacement: nothing in common, so each side is a single
			// Changed segment.
			name:   "no common tokens",
			old:    "abc",
			new:    "xyz",
			wantOK: true,
			wantOld: []Segment{
				{Text: "abc", Changed: true},
			},
			wantNew: []Segment{
				{Text: "xyz", Changed: true},
			},
		},
		{
			// Multibyte / Japanese: spaceless CJK is tokenized per character, so
			// the single changed character is isolated while the shared head and
			// tail stay unchanged — the whole point of #161. (Previously the lack
			// of an alignable boundary marked the entire line changed.)
			name:   "japanese changed word",
			old:    "私は猫が好きです",
			new:    "私は犬が好きです",
			wantOK: true,
			wantOld: []Segment{
				{Text: "私は", Changed: false},
				{Text: "猫", Changed: true},
				{Text: "が好きです", Changed: false},
			},
			wantNew: []Segment{
				{Text: "私は", Changed: false},
				{Text: "犬", Changed: true},
				{Text: "が好きです", Changed: false},
			},
		},
		{
			// With spaces as token separators the CJK words align on the shared
			// runs, isolating the single differing word on each side.
			name:   "japanese spaced changed word",
			old:    "私 は 猫 が 好き",
			new:    "私 は 犬 が 好き",
			wantOK: true,
			wantOld: []Segment{
				{Text: "私 は ", Changed: false},
				{Text: "猫", Changed: true},
				{Text: " が 好き", Changed: false},
			},
			wantNew: []Segment{
				{Text: "私 は ", Changed: false},
				{Text: "犬", Changed: true},
				{Text: " が 好き", Changed: false},
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Diff(c.old, c.new)
			if ok != c.wantOK {
				t.Fatalf("Diff(%q,%q) ok = %v, want %v", c.old, c.new, ok, c.wantOK)
			}
			if !c.wantOK {
				if got != nil {
					t.Fatalf("Diff(%q,%q) = %#v, want nil", c.old, c.new, got)
				}
				return
			}
			if !equalSegs(got.Old, c.wantOld) {
				t.Fatalf("Diff(%q,%q).Old = %#v, want %#v", c.old, c.new, segTuples(got.Old), c.wantOld)
			}
			if !equalSegs(got.New, c.wantNew) {
				t.Fatalf("Diff(%q,%q).New = %#v, want %#v", c.old, c.new, segTuples(got.New), c.wantNew)
			}
			// The segments must reconstruct the original inputs exactly.
			if s := joinSegments(got.Old); s != c.old {
				t.Fatalf("join(Old) = %q, want %q", s, c.old)
			}
			if s := joinSegments(got.New); s != c.new {
				t.Fatalf("join(New) = %q, want %q", s, c.new)
			}
			// No two adjacent segments may share a Changed flag (pushDiffPart
			// must have merged them).
			assertMerged(t, got.Old)
			assertMerged(t, got.New)
		})
	}
}

func assertMerged(t *testing.T, segs []Segment) {
	t.Helper()
	for i := 1; i < len(segs); i++ {
		if segs[i-1].Changed == segs[i].Changed {
			t.Fatalf("adjacent segments share Changed=%v: %#v", segs[i].Changed, segs)
		}
	}
	for _, s := range segs {
		if s.Text == "" {
			t.Fatalf("empty segment in %#v", segs)
		}
	}
}

func TestDiffOverCharLimit(t *testing.T) {
	t.Parallel()
	// Combined rune count exceeds MaxChars, so Diff bails out even though the
	// two strings differ.
	old := strings.Repeat("a", MaxChars)
	new := strings.Repeat("b", MaxChars)
	if got, ok := Diff(old, new); ok || got != nil {
		t.Fatalf("Diff over MaxChars = (%#v,%v), want (nil,false)", got, ok)
	}
}

func TestDiffOverTokenLimit(t *testing.T) {
	t.Parallel()
	// Stay under MaxChars but exceed MaxTokens: alternating word/space tokens
	// make each " a" contribute two tokens. MaxTokens/2 + 1 words on each side
	// pushes the combined token count past MaxTokens while keeping runes low.
	words := MaxTokens/2 + 1
	old := strings.TrimSpace(strings.Repeat("a ", words))
	new := strings.TrimSpace(strings.Repeat("b ", words))
	// Sanity: inputs stay within the char guard so the token guard is what
	// trips.
	if len([]rune(old))+len([]rune(new)) > MaxChars {
		t.Fatalf("test setup exceeded MaxChars unexpectedly")
	}
	if got, ok := Diff(old, new); ok || got != nil {
		t.Fatalf("Diff over MaxTokens = (%#v,%v), want (nil,false)", got, ok)
	}
}

func TestDiffEmptyToNonEmpty(t *testing.T) {
	t.Parallel()
	// One side empty: everything on the other side is a single Changed run,
	// and the empty side has no segments.
	got, ok := Diff("", "hello world")
	if !ok {
		t.Fatalf("Diff(\"\",...) ok = false, want true")
	}
	if len(got.Old) != 0 {
		t.Fatalf("Old = %#v, want empty", got.Old)
	}
	want := []Segment{{Text: "hello world", Changed: true}}
	if !equalSegs(got.New, want) {
		t.Fatalf("New = %#v, want %#v", got.New, want)
	}
}
