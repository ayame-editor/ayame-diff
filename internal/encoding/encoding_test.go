package encoding

import (
	"bytes"
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

// TestDetectISO2022JP covers #158: ISO-2022-JP is 7-bit so it used to be
// misreported as UTF-8 (rendering raw ESC sequences); auto-detect must now
// recognize its charset-designation escapes and round-trip.
func TestDetectISO2022JP(t *testing.T) {
	t.Parallel()
	const jp = "こんにちは、世界"
	raw := encodeTo(t, jp, japanese.ISO2022JP)
	if got := Detect(raw, Auto); got != ISO2022JP {
		t.Fatalf("Detect(ISO-2022-JP bytes) = %q, want %q", got, ISO2022JP)
	}
	if got := decodeAll(t, raw, Detect(raw, Auto)); got != jp {
		t.Errorf("round-trip = %q, want %q", got, jp)
	}
}

// TestDetectBOMlessUTF16 covers #158: without a BOM, ASCII-heavy UTF-16 used to
// fall through to the Shift_JIS/EUC-JP tie-breaker and decode as garbage. It is
// now recognized from the NUL byte parity, both little- and big-endian.
func TestDetectBOMlessUTF16(t *testing.T) {
	t.Parallel()
	const s = "Hello, world 123\nsecond line"
	for _, c := range []struct {
		name string
		enc  xencoding.Encoding
		want string
	}{
		{"LE", unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), UTF16LE},
		{"BE", unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), UTF16BE},
	} {
		raw := encodeTo(t, s, c.enc)
		if got := Detect(raw, Auto); got != c.want {
			t.Errorf("%s: Detect(BOM-less UTF-16) = %q, want %q", c.name, got, c.want)
		}
		if got := decodeAll(t, raw, Detect(raw, Auto)); got != s {
			t.Errorf("%s: round-trip = %q, want %q", c.name, got, s)
		}
	}
}

// TestJapaneseNotMisdetectedAsUTF16 guards the heuristic's precision: Shift_JIS
// and EUC-JP text carries essentially no NUL bytes, so the BOM-less UTF-16
// detector must never fire on it.
func TestJapaneseNotMisdetectedAsUTF16(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		enc  xencoding.Encoding
		want string
	}{
		{"shift_jis", japanese.ShiftJIS, ShiftJIS},
		{"euc-jp", japanese.EUCJP, EUCJP},
	} {
		raw := encodeTo(t, "日本語のテキスト表示", c.enc)
		if got := Detect(raw, Auto); got != c.want {
			t.Errorf("%s misdetected as %q, want %q", c.name, got, c.want)
		}
	}
}

// TestEncoderRoundTrips confirms Encoder is the inverse of Decoder for every
// concrete codec: UTF-8 text encoded to a target and decoded back is unchanged.
func TestEncoderRoundTrips(t *testing.T) {
	t.Parallel()
	const s = "hello 日本語 123\nsecond ライン"
	for _, name := range []string{UTF8, UTF16LE, UTF16BE, ShiftJIS, EUCJP, ISO2022JP} {
		var buf bytes.Buffer
		if _, err := io.WriteString(Encoder(&buf, name), s); err != nil {
			t.Fatalf("%s: encode: %v", name, err)
		}
		if got := decodeAll(t, buf.Bytes(), name); got != s {
			t.Errorf("%s: round-trip = %q, want %q", name, got, s)
		}
	}
}

// TestEncoderUTF8IsPassthrough documents that the UTF-8 encoder neither
// transforms bytes nor prepends a BOM, so a UTF-8 BOM is the caller's job.
func TestEncoderUTF8IsPassthrough(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w := Encoder(&buf, UTF8)
	if w != &buf {
		t.Fatalf("UTF-8 Encoder wrapped the writer; want passthrough")
	}
	io.WriteString(w, "abc")
	if got := buf.Bytes(); !bytes.Equal(got, []byte("abc")) {
		t.Errorf("UTF-8 encode = % x, want 61 62 63", got)
	}
}

// TestEncoderUTF16EmitsBOM confirms the UTF-16 encoders write their own
// byte-order mark, so the merge path must not add one for those encodings.
func TestEncoderUTF16EmitsBOM(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		bom  []byte
	}{
		{UTF16LE, []byte{0xFF, 0xFE}},
		{UTF16BE, []byte{0xFE, 0xFF}},
	} {
		var buf bytes.Buffer
		io.WriteString(Encoder(&buf, c.name), "A")
		if got := buf.Bytes(); len(got) < 2 || !bytes.Equal(got[:2], c.bom) {
			t.Errorf("%s: leading bytes = % x, want BOM % x", c.name, got, c.bom)
		}
	}
}

// TestEncoderRejectsUnrepresentable verifies an unmappable rune surfaces as a
// write error rather than a silent substitution, so merges never persist
// mojibake.
func TestEncoderRejectsUnrepresentable(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// 🍣 has no Shift_JIS mapping.
	_, err := io.WriteString(Encoder(&buf, ShiftJIS), "a🍣b")
	if err == nil {
		t.Fatal("encoding an unrepresentable rune to Shift_JIS did not error")
	}
}
