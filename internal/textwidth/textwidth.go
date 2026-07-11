// Package textwidth provides terminal-cell width helpers for Japanese/CJK text
// and column alignment. East Asian width properties come from x/text, which is
// already used by the input encoding layer.
//
// It was extracted from the (now removed) interactive TUI and is retained for
// side-by-side diff output (see hjosugi/ayame-diff#6).
package textwidth

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

// Options controls display-width choices that depend on the terminal locale.
type Options struct {
	// EastAsianAmbiguousWide counts East Asian Ambiguous characters (for
	// example ○, ※, and Greek letters) as two cells. They remain one cell by
	// default, matching most non-CJK terminals.
	EastAsianAmbiguousWide bool
}

// DisplayWidth returns the number of terminal cells s occupies.
func DisplayWidth(s string) int {
	return DisplayWidthWithOptions(s, Options{})
}

// DisplayWidthWithOptions returns the number of terminal cells s occupies
// using the requested locale-dependent width choices.
func DisplayWidthWithOptions(s string, opts Options) int {
	runes := []rune(s)
	width := 0
	for i := 0; i < len(runes); {
		var clusterWidth int
		i, clusterWidth = nextCluster(runes, i, opts)
		width += clusterWidth
	}
	return width
}

// Truncate shortens s to at most maxWidth display cells, appending "..." when
// it must cut (or "." repeated when maxWidth is too small for an ellipsis).
func Truncate(s string, maxWidth int) string {
	return TruncateWithOptions(s, maxWidth, Options{})
}

// TruncateWithOptions is Truncate using the requested locale-dependent width
// choices. It never cuts a combining or emoji ZWJ sequence in half.
func TruncateWithOptions(s string, maxWidth int, opts Options) string {
	if maxWidth <= 0 {
		return ""
	}
	if DisplayWidthWithOptions(s, opts) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	target := maxWidth - 3
	var b strings.Builder
	cellWidth := 0
	runes := []rune(s)
	for i := 0; i < len(runes); {
		next, clusterWidth := nextCluster(runes, i, opts)
		if cellWidth+clusterWidth > target {
			break
		}
		for _, r := range runes[i:next] {
			b.WriteRune(r)
		}
		cellWidth += clusterWidth
		i = next
	}
	b.WriteString("...")
	return b.String()
}

// PadRight pads s with spaces to exactly width display cells, truncating when s
// is wider.
func PadRight(s string, width int) string {
	return PadRightWithOptions(s, width, Options{})
}

// PadRightWithOptions is PadRight using the requested locale-dependent width
// choices.
func PadRightWithOptions(s string, width int, opts Options) string {
	current := DisplayWidthWithOptions(s, opts)
	if current >= width {
		return TruncateWithOptions(s, width, opts)
	}
	return s + strings.Repeat(" ", width-current)
}

// Inline escapes record separators and control characters so a legal CSV/TSV
// field always occupies a single row.
func Inline(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if unicode.IsControl(r) {
				_, _ = fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func runeDisplayWidth(r rune, opts Options) int {
	if isZeroWidth(r) {
		return 0
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	case width.EastAsianAmbiguous:
		if opts.EastAsianAmbiguousWide {
			return 2
		}
	}
	return 1
}

func isZeroWidth(r rune) bool {
	return r == 0 ||
		unicode.Is(unicode.Mn, r) ||
		unicode.Is(unicode.Me, r) ||
		unicode.Is(unicode.Cf, r) ||
		// Hangul medial/final Jamo combine with a leading consonant. Their
		// Unicode category is Lo, so the generic mark checks above miss them.
		(r >= 0x1160 && r <= 0x11ff) ||
		(r >= 0xd7b0 && r <= 0xd7ff)
}

// nextCluster returns the first rune after a display cluster and its cell
// width. This is intentionally a terminal-oriented subset of Unicode grapheme
// breaking: it keeps combining marks, emoji modifiers, regional-indicator
// flags, and ZWJ emoji together. That prevents family emoji from being counted
// once per person and keeps truncation from emitting a broken sequence.
func nextCluster(runes []rune, start int, opts Options) (int, int) {
	i := start + 1
	clusterWidth := runeDisplayWidth(runes[start], opts)
	regionalPair := isRegionalIndicator(runes[start])
	if regionalPair && i < len(runes) && isRegionalIndicator(runes[i]) {
		clusterWidth = 2
		i++
	}

	for {
		for i < len(runes) && isClusterExtender(runes[i]) {
			if runes[i] == '\ufe0f' || runes[i] == '\u20e3' {
				clusterWidth = max(clusterWidth, 2)
			}
			i++
		}
		if i+1 >= len(runes) || runes[i] != '\u200d' {
			break
		}
		i++ // zero-width joiner
		clusterWidth = max(clusterWidth, runeDisplayWidth(runes[i], opts))
		i++
	}
	return i, clusterWidth
}

func isClusterExtender(r rune) bool {
	// ZWJ terminates the current extension run and joins the next base rune;
	// consuming it here would split the sequence at the following emoji.
	return r != '\u200d' && (isZeroWidth(r) || isEmojiModifier(r))
}

func isEmojiModifier(r rune) bool {
	return r >= 0x1f3fb && r <= 0x1f3ff
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1f1e6 && r <= 0x1f1ff
}
