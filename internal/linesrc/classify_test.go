package linesrc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// TestOpenRejectsDirectoryAndBinary covers #166: the text/line reader steers
// directory and binary inputs to a clear, mode-naming error instead of a raw
// "is a directory" syscall message or a mojibake decode. Real text — including
// NUL-bearing UTF-16 — still opens.
func TestOpenRejectsDirectoryAndBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if _, err := OpenEncoding(dir, "auto"); !errors.Is(err, ErrIsDirectory) {
		t.Fatalf("directory: err = %v, want ErrIsDirectory", err)
	}

	binPath := filepath.Join(dir, "data.bin")
	// A PNG-like header: high bytes and a lone NUL — clearly not text, and its
	// low NUL density does not resemble UTF-16.
	binary := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x0d, 'I', 'H', 'D', 'R', 0xde, 0xad, 0xbe, 0xef}
	if err := os.WriteFile(binPath, binary, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenEncoding(binPath, "auto"); !errors.Is(err, ErrBinaryContent) {
		t.Fatalf("binary: err = %v, want ErrBinaryContent", err)
	}

	// UTF-16 text carries NUL bytes but is not binary.
	u16, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder(), []byte("日本語\nsecond\n"))
	if err != nil {
		t.Fatal(err)
	}
	u16Path := filepath.Join(dir, "u16.txt")
	if err := os.WriteFile(u16Path, u16, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := OpenEncoding(u16Path, "auto")
	if err != nil {
		t.Fatalf("utf-16 text misdetected as binary: %v", err)
	}
	f.Close()

	// Plain UTF-8 text opens normally.
	txtPath := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(txtPath, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err = OpenEncoding(txtPath, "auto")
	if err != nil {
		t.Fatalf("plain text rejected: %v", err)
	}
	f.Close()
}
