package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadAllBoundedRefusesOversizedPipes is the #137 regression for the paths
// that cannot stream: stdin and a --pre command are pipes whose length is
// unknowable in advance, so their content must be materialized. They must
// refuse with guidance rather than being OOM-killed.
func TestReadAllBoundedRefusesOversizedPipes(t *testing.T) {
	original := maxPipedInputBytes
	t.Cleanup(func() { maxPipedInputBytes = original })
	maxPipedInputBytes = 16

	if _, err := readAllBounded(strings.NewReader(strings.Repeat("a", 17)), "standard input"); err == nil {
		t.Fatal("an oversized pipe was accepted")
	} else {
		if !strings.Contains(err.Error(), "standard input") {
			t.Errorf("message %q must name the source", err.Error())
		}
		// A file argument streams, so that is the way forward.
		if !strings.Contains(err.Error(), "file") {
			t.Errorf("message %q must point at using a file instead", err.Error())
		}
	}

	// Exactly at the limit is fine; the cap is inclusive.
	data, err := readAllBounded(strings.NewReader(strings.Repeat("a", 16)), "standard input")
	if err != nil {
		t.Fatalf("input of exactly the limit was rejected: %v", err)
	}
	if len(data) != 16 {
		t.Fatalf("len=%d want 16", len(data))
	}
}

// TestSortedRejectsAnOverlongLine drives the CLI end to end: a file with no
// line breaks must fail at open with the runtime code, not be read into memory.
func TestSortedRejectsAnOverlongLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	big, small := filepath.Join(dir, "big.txt"), filepath.Join(dir, "small.txt")
	if err := os.WriteFile(big, []byte(strings.Repeat("a", 4096)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runText([]string{"--max-line-bytes", "1KiB", big, small}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("code=%d want %d (runtime)\nstderr=%q", code, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--max-line-bytes") {
		t.Errorf("stderr=%q must name the flag that adjusts the limit", stderr.String())
	}
}

// TestMaxLineBytesRejectsBadValues keeps a malformed size a usage error, not a
// runtime one, so the #113 taxonomy holds.
func TestMaxLineBytesRejectsBadValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	for _, path := range []string{a, b} {
		if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runText([]string{"--max-line-bytes", "not-a-size", a, b}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("code=%d want %d (usage)\nstderr=%q", code, exitUsage, stderr.String())
	}
}

// TestMaxLineBytesZeroDisablesTheCheck keeps the escape hatch reachable from
// the command line.
func TestMaxLineBytesZeroDisablesTheCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	big, small := filepath.Join(dir, "big.txt"), filepath.Join(dir, "small.txt")
	if err := os.WriteFile(big, []byte(strings.Repeat("a", 4096)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte(strings.Repeat("a", 4096)), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runText([]string{"--max-line-bytes", "0", "--summary", big, small}, &stdout, &stderr); code != exitOK {
		t.Fatalf("code=%d want %d\nstderr=%q", code, exitOK, stderr.String())
	}
}
