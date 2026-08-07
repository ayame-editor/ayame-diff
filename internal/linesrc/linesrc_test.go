package linesrc

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

// FileLines must satisfy the interface the diff walk consumes.
var _ linediff.Lines = (*FileLines)(nil)

// writeFile writes content to a temp file; when name ends in ".gz" the bytes
// are gzip-compressed so the file exercises the decompression path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := []byte(content)
	if strings.HasSuffix(name, ".gz") {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("gzip write: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("gzip close: %v", err)
		}
		data = buf.Bytes()
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// assertMatchesSplitLines opens path and verifies it serves exactly the lines
// linediff.SplitLines(content) would, plus correct out-of-range behavior.
func assertMatchesSplitLines(t *testing.T, path, content string) {
	t.Helper()
	want := linediff.SplitLines(content)

	f, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%s): %v", path, err)
	}
	defer f.Close()

	if got := f.Count(); got != uint64(len(want)) {
		t.Fatalf("Count() = %d, want %d (content %q)", got, len(want), content)
	}
	// Forward iteration over every valid index.
	for i := range want {
		got, ok := f.Line(uint64(i))
		if !ok || got != want[i] {
			t.Fatalf("Line(%d) = (%q,%v), want (%q,true) (content %q)", i, got, ok, want[i], content)
		}
	}
	// One past the end and well past the end both report absent.
	if got, ok := f.Line(uint64(len(want))); ok || got != "" {
		t.Fatalf("Line(Count) = (%q,%v), want (\"\",false)", got, ok)
	}
	if got, ok := f.Line(uint64(len(want)) + 5); ok || got != "" {
		t.Fatalf("Line(Count+5) = (%q,%v), want (\"\",false)", got, ok)
	}
}

func TestLineSemanticsMatchSplitLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"trailing_newline", "a\nb\nc\n"},
		{"no_trailing_newline", "a\nb\nc"},
		{"single_line_no_newline", "hello"},
		{"crlf", "a\r\nb\r\nc\r\n"},
		{"crlf_no_trailing", "a\r\nb\r\nc"},
		{"cr_only", "a\rb\rc\r"},
		{"cr_only_no_trailing", "a\rb\rc"},
		{"lone_newline", "\n"},
		{"embedded_blank", "a\n\nb\n"},
		{"only_blanks", "\n\n\n"},
		{"leading_blank", "\n\na\n"},
		{"mixed_endings", "a\r\nb\nc\rd\r\n"},
		{"trailing_cr_no_newline", "a\r"},
		{"unicode", "あいう\n😀\nmixed幅\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			// Plain and gzip must both match SplitLines exactly.
			assertMatchesSplitLines(t, writeFile(t, dir, "plain.txt", c.content), c.content)
			assertMatchesSplitLines(t, writeFile(t, dir, "compressed.txt.gz", c.content), c.content)
		})
	}
}

func TestFileLinesPreservesLineEndings(t *testing.T) {
	t.Parallel()
	path := writeFile(t, t.TempDir(), "mixed.txt", "lf\ncrlf\r\ncr\rfinal")
	lines, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lines.Close()
	for i, want := range []string{"\n", "\r\n", "\r", ""} {
		if got := lines.LineEnding(uint64(i)); got != want {
			t.Errorf("ending %d = %q, want %q", i, got, want)
		}
	}
}

// TestFileLinesReportsUTF8BOM verifies HasBOM reflects a stripped leading
// UTF-8 byte-order mark without leaking it into the first line (#159), and
// stays correct across a rewind that re-opens the stream.
func TestFileLinesReportsUTF8BOM(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	withBOM, err := Open(writeFile(t, dir, "bom.txt", "\uFEFFalpha\nbeta\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer withBOM.Close()
	if !withBOM.HasBOM() {
		t.Error("HasBOM() = false for a file with a UTF-8 BOM")
	}
	if line, _ := withBOM.Line(0); line != "alpha" {
		t.Errorf("first line = %q, want %q (BOM leaked)", line, "alpha")
	}
	// A back-reference below the window forces a reset; HasBOM must survive it.
	withBOM.Line(1)
	if line, _ := withBOM.Line(0); line != "alpha" || !withBOM.HasBOM() {
		t.Errorf("after rewind: line=%q hasBOM=%v", line, withBOM.HasBOM())
	}

	without, err := Open(writeFile(t, dir, "plain.txt", "alpha\nbeta\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer without.Close()
	if without.HasBOM() {
		t.Error("HasBOM() = true for a file without a BOM")
	}
}

// TestLongLineExceedsReaderBuffer proves lines far larger than the bufio buffer
// work (a bufio.Scanner would fail here).
func TestLongLineExceedsReaderBuffer(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 5*readerBufSize+123)
	content := "short\n" + long + "\ntail\n"
	dir := t.TempDir()
	assertMatchesSplitLines(t, writeFile(t, dir, "long.txt", content), content)
	assertMatchesSplitLines(t, writeFile(t, dir, "long.txt.gz", content), content)
}

func TestCRLFAtReaderBufferBoundary(t *testing.T) {
	t.Parallel()
	content := strings.Repeat("x", readerBufSize-1) + "\r\ntail\r"
	assertMatchesSplitLines(t, writeFile(t, t.TempDir(), "boundary.txt", content), content)
}

// TestRepeatedAndSequentialAccess covers cache hits and steady forward walking.
func TestRepeatedAndSequentialAccess(t *testing.T) {
	t.Parallel()
	content := "one\ntwo\nthree\nfour\nfive\n"
	want := linediff.SplitLines(content)
	dir := t.TempDir()
	f, err := Open(writeFile(t, dir, "seq.txt", content))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	// Sequential forward iteration.
	for i := range want {
		if got, _ := f.Line(uint64(i)); got != want[i] {
			t.Fatalf("seq Line(%d) = %q, want %q", i, got, want[i])
		}
	}
	// Re-reading an index inside the retained window returns the same value.
	for i := range want {
		if got, ok := f.Line(uint64(i)); !ok || got != want[i] {
			t.Fatalf("cached Line(%d) = (%q,%v), want %q", i, got, ok, want[i])
		}
	}
}

func genLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%08d-%s", i, strings.Repeat("z", i%17))
	}
	return lines
}

// TestLargeFileBoundedBuffer iterates a 50k-line file forward and asserts the
// sliding window stays bounded (never the whole file) while every line reads
// back correctly, then spot-checks a few indexes.
func TestLargeFileBoundedBuffer(t *testing.T) {
	t.Parallel()
	const n = 50000
	lines := genLines(n)
	content := strings.Join(lines, "\n") + "\n"

	for _, name := range []string{"big.txt", "big.txt.gz"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			f, err := Open(writeFile(t, dir, name, content))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer f.Close()

			if f.Count() != n {
				t.Fatalf("Count() = %d, want %d", f.Count(), n)
			}
			maxBuf := 0
			for i := 0; i < n; i++ {
				got, ok := f.Line(uint64(i))
				if !ok || got != lines[i] {
					t.Fatalf("Line(%d) = (%q,%v), want %q", i, got, ok, lines[i])
				}
				if len(f.buf) > maxBuf {
					maxBuf = len(f.buf)
				}
			}
			// Bounded memory: the window never exceeds the high-water mark, and
			// far fewer than all n lines are ever resident.
			if maxBuf > highWater {
				t.Fatalf("window grew to %d lines, exceeds highWater %d", maxBuf, highWater)
			}
			if uint64(maxBuf) >= n {
				t.Fatalf("window held %d lines; expected far fewer than %d", maxBuf, n)
			}
			// Dropping actually happened: the window base advanced off zero.
			if f.bufStart == 0 {
				t.Fatalf("bufStart still 0; no lines were dropped")
			}
			// Spot-check a few indexes still inside the retained window.
			for _, i := range []uint64{n - 1, n - 100, n - uint64(keepBehind)} {
				if got, ok := f.Line(i); !ok || got != lines[i] {
					t.Fatalf("spot Line(%d) = (%q,%v), want %q", i, got, ok, lines[i])
				}
			}
		})
	}
}

// TestBackwardAccessBelowWindow exercises the documented recovery path: an
// index that fell out of the retained window is served correctly via rescan.
func TestBackwardAccessBelowWindow(t *testing.T) {
	t.Parallel()
	const n = 20000
	lines := genLines(n)
	content := strings.Join(lines, "\n") + "\n"
	dir := t.TempDir()
	f, err := Open(writeFile(t, dir, "back.txt", content))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	// Walk to the far end so the early lines are dropped from the window.
	if got, ok := f.Line(n - 1); !ok || got != lines[n-1] {
		t.Fatalf("Line(%d) = (%q,%v), want %q", n-1, got, ok, lines[n-1])
	}
	if f.bufStart == 0 {
		t.Fatalf("expected early lines to be dropped before the backward read")
	}
	// Ask for an index far below the window: must rescan and return correctly.
	const back = 3
	if got, ok := f.Line(back); !ok || got != lines[back] {
		t.Fatalf("backward Line(%d) = (%q,%v), want %q", back, got, ok, lines[back])
	}
	// And forward progress from there still works.
	if got, ok := f.Line(back + 1); !ok || got != lines[back+1] {
		t.Fatalf("Line(%d) after rewind = (%q,%v), want %q", back+1, got, ok, lines[back+1])
	}
}

func TestOpenMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := Open(filepath.Join(t.TempDir(), "does-not-exist.txt")); err == nil {
		t.Fatal("Open of missing file returned nil error")
	}
}

func TestOpenBadGzip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bogus.gz")
	if err := os.WriteFile(path, []byte("not gzip data at all"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("Open of non-gzip .gz file returned nil error")
	}
}
