package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDirTSVAndDiffExitCode(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(oldDir, "a.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "a.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runDir([]string{"--tsv", "--include", "*.txt", "--workers", "2", "--diff-exit-code", oldDir, newDir}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stdout.String(), "status\tpath\told_size") || !strings.Contains(stdout.String(), "changed\ta.txt") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
