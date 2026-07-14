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

// TestNumericLessStrictWeakOrdering covers #165: numericLess must be a strict
// weak ordering even with NaN / ±Inf inputs. The old code returned false for
// both less(NaN,x) and less(x,NaN), which broke sort determinism.
func TestNumericLessStrictWeakOrdering(t *testing.T) {
	t.Parallel()
	sample := []string{"NaN", "nan", "Inf", "-Inf", "1e400", "-1e400", "0", "1", "2", "10", "1.0", "-3", "apple", "banana", ""}

	for _, a := range sample {
		if numericLess(a, a) {
			t.Errorf("numericLess(%q,%q) must be false (irreflexive)", a, a)
		}
		for _, b := range sample {
			if numericLess(a, b) && numericLess(b, a) {
				t.Errorf("asymmetry violated: less(%q,%q) and less(%q,%q)", a, b, b, a)
			}
		}
	}
	// less and its incomparability relation must both be transitive.
	equiv := func(a, b string) bool { return !numericLess(a, b) && !numericLess(b, a) }
	for _, a := range sample {
		for _, b := range sample {
			for _, c := range sample {
				if numericLess(a, b) && numericLess(b, c) && !numericLess(a, c) {
					t.Errorf("less not transitive: %q<%q<%q but not %q<%q", a, b, c, a, c)
				}
				if equiv(a, b) && equiv(b, c) && !equiv(a, c) {
					t.Errorf("incomparability not transitive: %q~%q~%q but not %q~%q", a, b, c, a, c)
				}
			}
		}
	}

	// A multiset with NaN sorts deterministically regardless of input order, and
	// NaN lands after the real numbers (which stay value-ordered).
	want := SortLines([]string{"NaN", "2", "1", "10"}, true, false)
	for _, perm := range [][]string{{"10", "1", "2", "NaN"}, {"1", "NaN", "10", "2"}, {"2", "10", "NaN", "1"}} {
		got := SortLines(perm, true, false)
		if len(got) != len(want) {
			t.Fatalf("length mismatch: %v vs %v", []string(got), []string(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("nondeterministic numeric sort: %v vs %v", []string(got), []string(want))
			}
		}
	}
	if want[0] != "1" || want[1] != "2" || want[2] != "10" || want[3] != "NaN" {
		t.Errorf("want 1,2,10,NaN, got %v", []string(want))
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
