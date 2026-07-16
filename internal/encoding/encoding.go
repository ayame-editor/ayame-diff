// Package encoding detects a text file's character encoding and provides a
// streaming decoder to UTF-8. It covers the encodings a Japanese-facing diff
// tool needs — UTF-8, UTF-16 (LE/BE, BOM-aware), Shift_JIS, EUC-JP, and
// ISO-2022-JP — mirroring WinMerge's codepage support (hjosugi/ayame-diff#9).
//
// This is the one place the project takes a dependency beyond the standard
// library: golang.org/x/text supplies the vetted Japanese codec tables, which
// are impractical to reproduce correctly by hand. See ADR 0003.
package encoding

import (
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Canonical encoding names accepted by Decoder and returned by Detect.
const (
	UTF8      = "utf-8"
	UTF16LE   = "utf-16le"
	UTF16BE   = "utf-16be"
	ShiftJIS  = "shift_jis"
	EUCJP     = "euc-jp"
	ISO2022JP = "iso-2022-jp"
	Auto      = "auto"
)

// Supported lists the selectable encodings (Auto plus the concrete codecs),
// for building CLI help and the GUI dropdown.
var Supported = []string{Auto, UTF8, UTF16LE, UTF16BE, ShiftJIS, EUCJP, ISO2022JP}

// Normalize maps common aliases to a canonical name; "" and unknown map to Auto.
func Normalize(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return Auto
	case "utf-8", "utf8":
		return UTF8
	case "utf-16", "utf16", "utf-16le", "utf16le", "unicode":
		return UTF16LE
	case "utf-16be", "utf16be":
		return UTF16BE
	case "shift_jis", "shift-jis", "shiftjis", "sjis", "cp932", "windows-31j", "ms932":
		return ShiftJIS
	case "euc-jp", "eucjp", "euc":
		return EUCJP
	case "iso-2022-jp", "iso2022jp", "jis":
		return ISO2022JP
	default:
		return Auto
	}
}

// codec returns the x/text encoding for a canonical name. UTF-8 returns nil
// (it needs no decoding). UTF-16 uses BOM-aware decoders.
func codec(name string) encoding.Encoding {
	switch name {
	case UTF16LE:
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	case UTF16BE:
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case ShiftJIS:
		return japanese.ShiftJIS
	case EUCJP:
		return japanese.EUCJP
	case ISO2022JP:
		return japanese.ISO2022JP
	default:
		return nil
	}
}

// Detect resolves the encoding of sample. hint is a user preference: a concrete
// name forces that encoding; Auto (or "") triggers detection from a byte-order
// mark and then a UTF-8 / Japanese heuristic. The returned name is always
// concrete (never Auto).
func Detect(sample []byte, hint string) string {
	// A byte-order mark is authoritative, even under an explicit hint.
	switch {
	case len(sample) >= 3 && sample[0] == 0xEF && sample[1] == 0xBB && sample[2] == 0xBF:
		return UTF8
	case len(sample) >= 2 && sample[0] == 0xFF && sample[1] == 0xFE:
		return UTF16LE
	case len(sample) >= 2 && sample[0] == 0xFE && sample[1] == 0xFF:
		return UTF16BE
	}
	if h := Normalize(hint); h != Auto {
		return h
	}
	// ISO-2022-JP is 7-bit (all bytes < 0x80), so it passes utf8.Valid and would
	// otherwise be misreported as UTF-8, showing raw escape sequences. Catch its
	// charset-designation escapes first.
	if looksISO2022JP(sample) {
		return ISO2022JP
	}
	// A BOM-less UTF-16 file has no byte-order mark to key on and its embedded
	// NUL bytes make utf8.Valid fail, so without this it would fall through to
	// the Shift_JIS/EUC-JP tie-breaker and decode as garbage. Recognize it from
	// the NUL byte parity instead.
	if enc := looksUTF16NoBOM(sample); enc != "" {
		return enc
	}
	if utf8.Valid(sample) {
		return UTF8
	}
	return detectJapanese(sample)
}

// looksISO2022JP reports whether sample contains an ISO-2022-JP escape that
// designates a JIS multi-byte set (ESC $ @ / ESC $ B / ESC $ ( ...). Those
// three-byte designators are the hallmark of Japanese ISO-2022-JP content and
// are vanishingly unlikely to appear by accident in UTF-8/ASCII text, so this
// avoids false positives on a stray ESC used for terminal control.
func looksISO2022JP(b []byte) bool {
	for i := 0; i+2 < len(b); i++ {
		if b[i] == 0x1B && b[i+1] == '$' {
			switch b[i+2] {
			case '@', 'B', '(': // JIS X 0208 (1978/1983) or JIS X 0212 via ESC $ (
				return true
			}
		}
	}
	return false
}

// looksUTF16NoBOM detects BOM-less UTF-16 from its NUL byte pattern: ASCII code
// units encode as (byte, 0x00) in little-endian and (0x00, byte) in big-endian,
// so NUL bytes cluster strongly on one parity. It requires NULs to be a
// substantial share of the sample (≥25%) and almost entirely single-parity, so
// Shift_JIS/EUC-JP text — which contains essentially no NUL bytes — never
// matches. Predominantly-CJK UTF-16 has few NULs and is not detected here; an
// explicit --encoding still forces it. Returns "" when the pattern is absent.
func looksUTF16NoBOM(b []byte) string {
	// Ignore a trailing odd byte so parity counts stay aligned to code units.
	n := len(b) &^ 1
	if n < 4 {
		return ""
	}
	var nul, nulEven, nulOdd int
	for i := 0; i < n; i++ {
		if b[i] == 0x00 {
			nul++
			if i%2 == 0 {
				nulEven++
			} else {
				nulOdd++
			}
		}
	}
	if nul*4 < n { // fewer than a quarter NUL bytes: not ASCII-heavy UTF-16
		return ""
	}
	switch {
	case nulOdd >= nul*9/10: // NULs on odd offsets -> (ascii, 00) -> little-endian
		return UTF16LE
	case nulEven >= nul*9/10: // NULs on even offsets -> (00, ascii) -> big-endian
		return UTF16BE
	default:
		return ""
	}
}

// detectJapanese chooses between Shift_JIS and EUC-JP by counting how many
// double-byte sequences fit each scheme's lead/trail byte ranges. It is a
// heuristic tie-breaker; users can always override with an explicit encoding.
func detectJapanese(b []byte) string {
	var sjis, euc int
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			i++
			continue
		}
		next := byte(0)
		if i+1 < len(b) {
			next = b[i+1]
		}
		// EUC-JP: two bytes in A1-FE, or 8E + A1-DF (half-width kana).
		if c >= 0xA1 && c <= 0xFE && next >= 0xA1 && next <= 0xFE {
			euc++
			i += 2
			continue
		}
		if c == 0x8E && next >= 0xA1 && next <= 0xDF {
			euc++
			i += 2
			continue
		}
		// Shift_JIS: lead 81-9F or E0-FC, trail 40-7E or 80-FC.
		if (c >= 0x81 && c <= 0x9F) || (c >= 0xE0 && c <= 0xFC) {
			if (next >= 0x40 && next <= 0x7E) || (next >= 0x80 && next <= 0xFC) {
				sjis++
				i += 2
				continue
			}
		}
		// Single-byte half-width kana (A1-DF) leans Shift_JIS.
		if c >= 0xA1 && c <= 0xDF {
			sjis++
		}
		i++
	}
	if euc > sjis {
		return EUCJP
	}
	return ShiftJIS
}

// Decoder wraps r so reads yield UTF-8, decoding from the given concrete
// encoding name (as returned by Detect). UTF-8 is returned unchanged.
func Decoder(r io.Reader, name string) io.Reader {
	enc := codec(name)
	if enc == nil {
		return r
	}
	return transform.NewReader(r, enc.NewDecoder())
}
