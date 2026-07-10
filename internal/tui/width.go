package tui

import (
	"fmt"
	"strings"
	"unicode"
)

// DisplayWidth is a deliberately small wcwidth-compatible implementation.
// It is sufficient for Japanese headers and paths without adding a runtime
// dependency to the native executable.
func DisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeDisplayWidth(r)
	}
	return width
}

func TruncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if DisplayWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	target := maxWidth - 3
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := runeDisplayWidth(r)
		if width+rw > target {
			break
		}
		b.WriteRune(r)
		width += rw
	}
	b.WriteString("...")
	return b.String()
}

func PadRightDisplay(s string, width int) string {
	current := DisplayWidth(s)
	if current >= width {
		return TruncateDisplay(s, width)
	}
	return s + strings.Repeat(" ", width-current)
}

// InlineText escapes record separators and control characters so a legal
// CSV/TSV header always occupies one terminal row.
func InlineText(s string) string {
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

func runeDisplayWidth(r rune) int {
	if r == 0 || r == '\u200d' || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	return r >= 0x1100 && (r <= 0x115f ||
		r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff) ||
		(r >= 0x20000 && r <= 0x3fffd))
}
