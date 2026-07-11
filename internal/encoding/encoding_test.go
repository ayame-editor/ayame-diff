package encoding

import (
	"io"
	"testing"

	xencoding "golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func encodeTo(t *testing.T, s string, enc xencoding.Encoding) []byte {
	t.Helper()
	b, _, err := transform.Bytes(enc.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return b
}

func decodeAll(t *testing.T, raw []byte, name string) string {
	t.Helper()
	out, err := io.ReadAll(Decoder(bytesReader(raw), name))
	if err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return string(out)
}

// bytesReader avoids importing bytes just for a reader in a couple of places.
func bytesReader(b []byte) io.Reader { return &sliceReader{b: b} }

type sliceReader struct {
	b []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

func TestNormalize(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"": Auto, "auto": Auto, "UTF-8": UTF8, "utf8": UTF8,
		"sjis": ShiftJIS, "Shift-JIS": ShiftJIS, "cp932": ShiftJIS,
		"euc-jp": EUCJP, "EUCJP": EUCJP, "iso-2022-jp": ISO2022JP,
		"utf-16": UTF16LE, "utf-16be": UTF16BE, "nonsense": Auto,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectAndDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	const jp = "日本語のテスト\nABC 123\n漢字とかな\n"

	cases := []struct {
		name string
		enc  xencoding.Encoding
		want string // expected auto-detected encoding
	}{
		{ShiftJIS, japanese.ShiftJIS, ShiftJIS},
		{EUCJP, japanese.EUCJP, EUCJP},
		{UTF16LE, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM), UTF16LE},
		{UTF16BE, unicode.UTF16(unicode.BigEndian, unicode.UseBOM), UTF16BE},
	}
	for _, c := range cases {
		raw := encodeTo(t, jp, c.enc)

		// Auto-detection resolves the right encoding.
		if got := Detect(raw, Auto); got != c.want {
			t.Errorf("Detect(%s bytes) = %q, want %q", c.name, got, c.want)
		}
		// Decoding with the resolved name recovers the original UTF-8.
		if got := decodeAll(t, raw, Detect(raw, Auto)); got != jp {
			t.Errorf("%s round-trip = %q, want %q", c.name, got, jp)
		}
		// An explicit hint also works.
		if got := decodeAll(t, raw, c.name); got != jp {
			t.Errorf("%s explicit decode = %q", c.name, got)
		}
	}
}

func TestDetectUTF8AndBOM(t *testing.T) {
	t.Parallel()
	if got := Detect([]byte("plain ascii\n"), Auto); got != UTF8 {
		t.Fatalf("ascii detect = %q", got)
	}
	if got := Detect([]byte("日本語 utf8\n"), Auto); got != UTF8 {
		t.Fatalf("utf8 detect = %q", got)
	}
	// UTF-8 BOM is authoritative even under a conflicting hint.
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte("x")...)
	if got := Detect(bom, ShiftJIS); got != UTF8 {
		t.Fatalf("utf8-BOM detect = %q, want utf-8", got)
	}
}

func TestExplicitHintForcesEncoding(t *testing.T) {
	t.Parallel()
	// Bytes valid as both; an explicit euc-jp hint must win over the heuristic.
	raw := encodeTo(t, "漢字", japanese.EUCJP)
	if got := Detect(raw, EUCJP); got != EUCJP {
		t.Fatalf("explicit euc-jp = %q", got)
	}
}
