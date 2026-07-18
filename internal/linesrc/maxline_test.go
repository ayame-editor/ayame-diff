package linesrc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOpenRejectsAnOverlongLine is the #137 regression. The sliding window
// bounds how many lines stay resident, but a file with no line breaks is one
// line, so the "streaming" reader held the whole file. The refusal must happen
// at open time, before any comparison starts.
func TestOpenRejectsAnOverlongLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "minified.json")
	// One line, no terminator anywhere — the shape of minified data.
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 5000)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := OpenEncodingLimit(path, "auto", 1024)
	if err == nil {
		t.Fatal("an overlong line was accepted")
	}
	var tooLong *LineTooLongError
	if !errors.As(err, &tooLong) {
		t.Fatalf("err=%T (%v) want *LineTooLongError", err, err)
	}
	if tooLong.Limit != 1024 {
		t.Errorf("limit=%d want 1024", tooLong.Limit)
	}
	// The message must point somewhere, not just refuse.
	for _, want := range []string{"--max-line-bytes", "binary (bin) mode"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err.Error(), want)
		}
	}
}

// TestOpenAcceptsLinesAtTheLimit keeps the boundary inclusive, so a limit
// expressed as "this many bytes" admits a line of exactly that size.
func TestOpenAcceptsLinesAtTheLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "exact.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 1024)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := OpenEncodingLimit(path, "auto", 1024)
	if err != nil {
		t.Fatalf("a line of exactly the limit was rejected: %v", err)
	}
	defer file.Close()
	if file.Count() != 1 {
		t.Fatalf("count=%d want 1", file.Count())
	}
}

// TestMaxLineLimitCountsOneLineNotTheBuffer guards the check against firing on
// a run of short CR-delimited lines, which share one buffered chunk.
func TestMaxLineLimitCountsOneLineNotTheBuffer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cr.txt")
	// 4000 bytes total, but every line is 3 bytes.
	if err := os.WriteFile(path, []byte(strings.Repeat("ab\r", 1000)), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := OpenEncodingLimit(path, "auto", 100)
	if err != nil {
		t.Fatalf("short CR-delimited lines tripped the per-line limit: %v", err)
	}
	defer file.Close()
	if file.Count() != 1000 {
		t.Fatalf("count=%d want 1000", file.Count())
	}
}

// TestZeroLimitDisablesTheCheck keeps an escape hatch for inputs the user knows
// are fine.
func TestZeroLimitDisablesTheCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "long.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 200000)), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := OpenEncodingLimit(path, "auto", 0)
	if err != nil {
		t.Fatalf("zero limit still rejected the file: %v", err)
	}
	defer file.Close()
	line, ok := file.Line(0)
	if !ok || len(line) != 200000 {
		t.Fatalf("line length=%d ok=%v want 200000", len(line), ok)
	}
}
