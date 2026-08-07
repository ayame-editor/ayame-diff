package threeway

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/encoding"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/linesrc"
)

// encodeBytes re-encodes UTF-8 s to the named encoding. UTF-16 codecs add their
// own BOM; a UTF-8 BOM is prepended by the caller.
func encodeBytes(t *testing.T, s, encName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := encoding.Encoder(&buf, encName)
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatalf("encode %s: %v", encName, err)
	}
	// Close finalizes stateful codecs (ISO-2022-JP's return-to-ASCII escape),
	// matching WriteMerged so the expected bytes are fully conformant.
	if c, ok := w.(io.Closer); ok {
		if err := c.Close(); err != nil {
			t.Fatalf("close %s: %v", encName, err)
		}
	}
	return buf.Bytes()
}

// rawFile serializes logical lines into the exact bytes a source/expected file
// should hold: joined by eol, optionally newline-terminated, optionally prefixed
// with a UTF-8 BOM, then encoded. It mirrors the round-trip spec independently
// of WriteMerged.
func rawFile(t *testing.T, lines []string, eol string, finalNL, utf8BOM bool, encName string) []byte {
	t.Helper()
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(line)
		if i < len(lines)-1 || finalNL {
			sb.WriteString(eol)
		}
	}
	body := encodeBytes(t, sb.String(), encName)
	if utf8BOM {
		body = append([]byte{0xEF, 0xBB, 0xBF}, body...)
	}
	return body
}

func writeRaw(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestMergeRoundTripsBaseConventions is the #159 regression: a written 3-way
// merge must reproduce the base file's encoding, BOM, line terminator, and
// final-newline state rather than normalizing to BOM-less UTF-8/LF with a
// forced trailing newline. Each case edits one line on the left only, so the
// merge auto-resolves and the output equals the left content in base's bytes.
func TestMergeRoundTripsBaseConventions(t *testing.T) {
	t.Parallel()
	base := []string{"alpha", "beta", "gamma"}
	left := []string{"alpha", "BETA", "gamma"}
	right := []string{"alpha", "beta", "gamma"}
	want := []string{"alpha", "BETA", "gamma"}

	jpBase := []string{"共通", "旧の行", "末尾"}
	jpLeft := []string{"共通", "新しい行", "末尾"}
	jpRight := []string{"共通", "旧の行", "末尾"}
	jpWant := []string{"共通", "新しい行", "末尾"}

	cases := []struct {
		name    string
		enc     string // --encoding hint fed to OpenEncoding
		encName string // codec for byte construction ("" == utf-8)
		eol     string
		finalNL bool
		utf8BOM bool
		b, l, r []string
		want    []string
	}{
		{"lf_trailing", "auto", "", "\n", true, false, base, left, right, want},
		{"crlf_trailing", "auto", "", "\r\n", true, false, base, left, right, want},
		{"lf_no_trailing", "auto", "", "\n", false, false, base, left, right, want},
		{"crlf_no_trailing", "auto", "", "\r\n", false, false, base, left, right, want},
		{"utf8_bom_lf", "auto", "", "\n", true, true, base, left, right, want},
		{"utf8_bom_crlf", "auto", "", "\r\n", true, true, base, left, right, want},
		{"shift_jis_crlf", encoding.ShiftJIS, encoding.ShiftJIS, "\r\n", true, false, jpBase, jpLeft, jpRight, jpWant},
		{"euc_jp_lf", encoding.EUCJP, encoding.EUCJP, "\n", true, false, jpBase, jpLeft, jpRight, jpWant},
		{"utf16le", encoding.UTF16LE, encoding.UTF16LE, "\n", true, false, jpBase, jpLeft, jpRight, jpWant},
		{"utf16be_crlf", encoding.UTF16BE, encoding.UTF16BE, "\r\n", true, false, jpBase, jpLeft, jpRight, jpWant},
		// ISO-2022-JP is stateful: WriteMerged must finalize the stream with a
		// return-to-ASCII escape, which matters most when the last line ends in
		// Japanese with no trailing newline.
		{"iso2022jp_lf", encoding.ISO2022JP, encoding.ISO2022JP, "\n", true, false, jpBase, jpLeft, jpRight, jpWant},
		{"iso2022jp_no_trailing", encoding.ISO2022JP, encoding.ISO2022JP, "\n", false, false, jpBase, jpLeft, jpRight, jpWant},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			basePath := filepath.Join(dir, "base")
			leftPath := filepath.Join(dir, "left")
			rightPath := filepath.Join(dir, "right")
			outPath := filepath.Join(dir, "merged")
			writeRaw(t, basePath, rawFile(t, c.b, c.eol, c.finalNL, c.utf8BOM, c.encName))
			writeRaw(t, leftPath, rawFile(t, c.l, c.eol, c.finalNL, c.utf8BOM, c.encName))
			writeRaw(t, rightPath, rawFile(t, c.r, c.eol, c.finalNL, c.utf8BOM, c.encName))

			baseLines, leftLines, rightLines := openTriple(t, c.enc, basePath, leftPath, rightPath)
			defer baseLines.Close()
			defer leftLines.Close()
			defer rightLines.Close()

			result, err := Compare(baseLines, leftLines, rightLines, linediff.Options{Window: 32})
			if err != nil {
				t.Fatal(err)
			}
			if result.Conflicts != 0 {
				t.Fatalf("unexpected conflicts: %+v", result.Events)
			}
			profile := ProfileOf(baseLines)
			merged, unresolved, err := MergeLines(baseLines, result, nil, false)
			if err != nil || unresolved != 0 {
				t.Fatalf("merge: unresolved=%d err=%v", unresolved, err)
			}
			if err := WriteMerged(outPath, merged, profile); err != nil {
				t.Fatalf("WriteMerged: %v", err)
			}

			got, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatal(err)
			}
			expected := rawFile(t, c.want, c.eol, c.finalNL, c.utf8BOM, c.encName)
			if !bytes.Equal(got, expected) {
				t.Fatalf("output bytes mismatch\n got: % x\nwant: % x", got, expected)
			}
			// Independent of the byte assertion: the output must re-open to the
			// same encoding, BOM, and terminators the base carried.
			assertRoundTrip(t, c.enc, outPath, c.want, c.eol, c.finalNL, c.utf8BOM)
		})
	}
}

func openTriple(t *testing.T, enc, basePath, leftPath, rightPath string) (*linesrc.FileLines, *linesrc.FileLines, *linesrc.FileLines) {
	t.Helper()
	base, err := linesrc.OpenEncoding(basePath, enc)
	if err != nil {
		t.Fatalf("open base: %v", err)
	}
	left, err := linesrc.OpenEncoding(leftPath, enc)
	if err != nil {
		t.Fatalf("open left: %v", err)
	}
	right, err := linesrc.OpenEncoding(rightPath, enc)
	if err != nil {
		t.Fatalf("open right: %v", err)
	}
	return base, left, right
}

// assertRoundTrip re-opens a written file and checks it decodes to wantLines
// with the expected terminator, final-newline state, and UTF-8 BOM.
func assertRoundTrip(t *testing.T, enc, path string, wantLines []string, eol string, finalNL, utf8BOM bool) {
	t.Helper()
	f, err := linesrc.OpenEncoding(path, enc)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer f.Close()
	if got := f.Count(); got != uint64(len(wantLines)) {
		t.Fatalf("line count = %d, want %d", got, len(wantLines))
	}
	for i, want := range wantLines {
		if got, _ := f.Line(uint64(i)); got != want {
			t.Errorf("line %d = %q, want %q", i, got, want)
		}
	}
	if f.HasBOM() != utf8BOM {
		t.Errorf("HasBOM() = %v, want %v", f.HasBOM(), utf8BOM)
	}
	// The final line's terminator encodes the final-newline state.
	last := f.Count() - 1
	wantEnding := ""
	if finalNL {
		wantEnding = eol
	}
	if got := f.LineEnding(last); got != wantEnding {
		t.Errorf("final terminator = %q, want %q", got, wantEnding)
	}
	if len(wantLines) > 1 {
		if got := f.LineEnding(0); got != eol {
			t.Errorf("first terminator = %q, want %q", got, eol)
		}
	}
}

// TestMergeShiftJISBytesAreExact pins a known Shift_JIS mapping so the output is
// verified against literal codec bytes, not just a decode round-trip.
func TestMergeShiftJISBytesAreExact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// "あ" is 82 a0 in Shift_JIS; the merged single line has no trailing newline.
	b := []string{"あ"}
	l := []string{"い"} // い is 82 a2
	r := []string{"あ"}
	writeRaw(t, filepath.Join(dir, "b"), rawFile(t, b, "\n", false, false, encoding.ShiftJIS))
	writeRaw(t, filepath.Join(dir, "l"), rawFile(t, l, "\n", false, false, encoding.ShiftJIS))
	writeRaw(t, filepath.Join(dir, "r"), rawFile(t, r, "\n", false, false, encoding.ShiftJIS))

	base, left, right := openTriple(t, encoding.ShiftJIS, filepath.Join(dir, "b"), filepath.Join(dir, "l"), filepath.Join(dir, "r"))
	defer base.Close()
	defer left.Close()
	defer right.Close()
	result, err := Compare(base, left, right, linediff.Options{Window: 8})
	if err != nil {
		t.Fatal(err)
	}
	profile := ProfileOf(base)
	merged, _, err := MergeLines(base, result, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := WriteMerged(out, merged, profile); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// い with no trailing newline, in Shift_JIS.
	want := []byte{0x82, 0xa2}
	if !bytes.Equal(got, want) {
		t.Fatalf("output = % x, want % x", got, want)
	}
}

// TestProfileOfInMemorySourceDefaults confirms a source without EOL/encoding
// metadata (in-memory SplitLines) keeps the historical UTF-8/LF/trailing
// defaults rather than panicking on the missing interfaces.
func TestProfileOfInMemorySourceDefaults(t *testing.T) {
	t.Parallel()
	profile := ProfileOf(linediff.SplitLines("a\nb\n"))
	if profile.Encoding != "" || profile.BOM || profile.LineEnding != "\n" || !profile.FinalNewline {
		t.Fatalf("in-memory profile = %+v, want UTF-8/LF/trailing defaults", profile)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := WriteMerged(out, []string{"a", "b"}, profile); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a\nb\n" {
		t.Fatalf("output = %q, want %q", got, "a\nb\n")
	}
}
