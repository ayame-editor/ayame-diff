package dircompare

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompareRefusesTooManyEntries is the #137 regression. A Result holds one
// Entry per file including unchanged ones, and the server copies the whole set
// again to serialize it, so an enormous tree allocated with nothing to stop it.
// The refusal must arrive before any per-file work and must say what to do.
func TestCompareRefusesTooManyEntries(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	for i := range 20 {
		name := fmt.Sprintf("f%02d.txt", i)
		for _, dir := range []string{oldDir, newDir} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err := CompareAny(oldDir, newDir, Options{CompareBy: CompareSize, MaxEntries: 5})
	if err == nil {
		t.Fatal("a tree past the entry limit was accepted")
	}
	if !errors.Is(err, ErrTooManyEntries) {
		t.Fatalf("err=%v want ErrTooManyEntries", err)
	}
	if !strings.Contains(err.Error(), "20") || !strings.Contains(err.Error(), "5") {
		t.Errorf("message %q must name the actual count and the limit", err.Error())
	}
	for _, want := range []string{"--include", "--max-entries"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err.Error(), want)
		}
	}
}

// TestCompareAllowsTreesWithinTheLimit keeps the boundary inclusive.
func TestCompareAllowsTreesWithinTheLimit(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	for i := range 5 {
		name := fmt.Sprintf("f%d.txt", i)
		for _, dir := range []string{oldDir, newDir} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	result, err := CompareAny(oldDir, newDir, Options{CompareBy: CompareSize, MaxEntries: 5})
	if err != nil {
		t.Fatalf("a tree exactly at the limit was rejected: %v", err)
	}
	if len(result.Entries) != 5 {
		t.Fatalf("entries=%d want 5", len(result.Entries))
	}
}

// TestNegativeEntryLimitDisablesTheCheck keeps an escape hatch.
func TestNegativeEntryLimitDisablesTheCheck(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	for i := range 10 {
		if err := os.WriteFile(filepath.Join(oldDir, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CompareAny(oldDir, newDir, Options{CompareBy: CompareSize, MaxEntries: -1}); err != nil {
		t.Fatalf("a negative limit still refused: %v", err)
	}
}
