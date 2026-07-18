package dircompare

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileComparePanicBecomesAnError is the #137 regression. File comparison
// runs on up to 64 worker goroutines, so a panic on one of them is unreachable
// from the caller's recover: before the guard a single malformed entry aborted
// the whole process instead of failing one comparison.
func TestFileComparePanicBecomesAnError(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	for _, dir := range []string{oldDir, newDir} {
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("same size"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	original := fileEqualFn
	fileEqualFn = func(context.Context, CompareMethod, source, source, string, Options) (bool, error) {
		panic("comparison worker exploded")
	}
	t.Cleanup(func() { fileEqualFn = original })

	_, err := CompareAny(oldDir, newDir, Options{CompareBy: CompareContents})
	if err == nil {
		t.Fatal("a panicking comparison worker did not fail the compare")
	}
	if !strings.Contains(err.Error(), "comparison worker exploded") {
		t.Fatalf("err=%v must name the panic so the bug stays reportable", err)
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Fatalf("err=%v must be marked as an internal error", err)
	}
}

// TestCompareSurvivesAWorkerPanic proves the process stays usable afterwards.
func TestCompareSurvivesAWorkerPanic(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	for _, dir := range []string{oldDir, newDir} {
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("same size"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	original := fileEqualFn
	fileEqualFn = func(context.Context, CompareMethod, source, source, string, Options) (bool, error) {
		panic("boom")
	}
	if _, err := CompareAny(oldDir, newDir, Options{CompareBy: CompareContents}); err == nil {
		t.Fatal("expected the panicking compare to fail")
	}
	fileEqualFn = original

	result, err := CompareAny(oldDir, newDir, Options{CompareBy: CompareContents})
	if err != nil {
		t.Fatalf("the compare after a panic failed: %v", err)
	}
	if len(result.Entries) == 0 {
		t.Error("the recovered process produced no comparison result")
	}
}
