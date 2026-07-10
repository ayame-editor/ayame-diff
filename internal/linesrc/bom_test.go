package linesrc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenStripsUTF8BOM(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// BOM followed by two lines.
	path := filepath.Join(dir, "bom.txt")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbfhello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if f.Count() != 2 {
		t.Fatalf("count = %d, want 2", f.Count())
	}
	if s, _ := f.Line(0); s != "hello" {
		t.Fatalf("line 0 = %q, want %q (BOM not stripped?)", s, "hello")
	}
	if s, _ := f.Line(1); s != "world" {
		t.Fatalf("line 1 = %q, want %q", s, "world")
	}

	// A file that is only a BOM must count as zero lines (count and stream must
	// agree, else Line(i) would index past a phantom line).
	only := filepath.Join(dir, "only-bom.txt")
	if err := os.WriteFile(only, []byte("\xef\xbb\xbf"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := Open(only)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if g.Count() != 0 {
		t.Fatalf("only-BOM count = %d, want 0", g.Count())
	}
	if _, ok := g.Line(0); ok {
		t.Fatal("only-BOM Line(0) should be out of range")
	}
}
