package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBinBoundsDenseDiffAndReportsTruncation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.bin")
	newPath := filepath.Join(dir, "new.bin")
	if err := os.WriteFile(oldPath, make([]byte, 128), 0o644); err != nil {
		t.Fatal(err)
	}
	newBytes := bytes.Repeat([]byte{0xff}, 128)
	if err := os.WriteFile(newPath, newBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runBin([]string{"--max-regions", "2", "--max-bytes", "8", oldPath, newPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if got := strings.Count(stdout.String(), "@ 0x"); got != 2 {
		t.Fatalf("regions in output = %d; output=%q", got, stdout.String())
	}
	if !strings.Contains(stderr.String(), "128 byte(s)") || !strings.Contains(stderr.String(), "truncated") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRunBinRejectsUnboundedLimits(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--max-regions", "0", "a", "b"}, {"--max-bytes", "0", "a", "b"}} {
		var stdout, stderr bytes.Buffer
		if code := runBin(args, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "must be at least 1") {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr.String())
		}
	}
}
