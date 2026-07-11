package linesort

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNumericLess(t *testing.T) {
	t.Parallel()
	if !numericLess("2", "10") {
		t.Fatal("numericLess(2,10) should be true (numeric)")
	}
	if numericLess("10", "2") {
		t.Fatal("numericLess(10,2) should be false")
	}
	if !numericLess("apple", "banana") {
		t.Fatal("non-numeric should fall back to lexical")
	}
	if numericLess("1.0", "1.0") {
		t.Fatal("equal values must not be less")
	}
}

func TestSorted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(path, []byte("cherry\napple\nbanana\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Sorted(path, false, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"apple", "banana", "cherry"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	rev, err := Sorted(path, false, true, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if rev[0] != "cherry" || rev[2] != "apple" {
		t.Fatalf("reverse sort = %v", rev)
	}

	num := filepath.Join(dir, "num.txt")
	if err := os.WriteFile(num, []byte("10\n2\n33\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ns, err := Sorted(num, true, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if ns[0] != "2" || ns[1] != "10" || ns[2] != "33" {
		t.Fatalf("numeric sort = %v", ns)
	}
}
