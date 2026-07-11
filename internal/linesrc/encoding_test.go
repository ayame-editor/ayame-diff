package linesrc

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// TestOpenDecodesShiftJIS proves the streaming path (detect + decode) recovers
// Japanese text from a Shift_JIS file, line by line, with bounded memory.
func TestOpenDecodesShiftJIS(t *testing.T) {
	t.Parallel()
	const utf8Text = "名前\t年齢\n田中\t30\n鈴木\t25\n"
	sjis, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(utf8Text))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sjis.txt")
	if err := os.WriteFile(path, sjis, 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Open(path) // auto-detect
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if f.Encoding() != "shift_jis" {
		t.Fatalf("detected encoding = %q, want shift_jis", f.Encoding())
	}
	want := []string{"名前\t年齢", "田中\t30", "鈴木\t25"}
	if f.Count() != uint64(len(want)) {
		t.Fatalf("count = %d, want %d", f.Count(), len(want))
	}
	for i, w := range want {
		if got, _ := f.Line(uint64(i)); got != w {
			t.Fatalf("line %d = %q, want %q", i, got, w)
		}
	}
}
