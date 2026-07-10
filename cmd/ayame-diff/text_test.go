package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/diffout"
)

func TestSubcommandDispatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"text", "a", "b"}, "text"},
		{[]string{"sorted", "a", "b"}, "sorted"},
		{[]string{"csv", "--left", "a"}, "csv"},
		{[]string{"--left", "a.csv"}, ""}, // bare flags => CSV back-compat
		{[]string{"--version"}, ""},       // handled before dispatch
		{[]string{"unknown", "x"}, ""},    // not a subcommand => CSV
	}
	for _, c := range cases {
		if got := subcommand(c.args); got != c.want {
			t.Fatalf("subcommand(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestDiffFlagsFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		d    diffFlags
		want diffout.Format
	}{
		{diffFlags{}, diffout.Unified},
		{diffFlags{side: true}, diffout.SideBySide},
		{diffFlags{summary: true}, diffout.Summary},
		{diffFlags{json: true}, diffout.JSON},
		{diffFlags{json: true, side: true}, diffout.JSON}, // json wins
		{diffFlags{summary: true, side: true}, diffout.Summary},
	}
	for _, c := range cases {
		if got := c.d.format(); got != c.want {
			t.Fatalf("format(%+v) = %v, want %v", c.d, got, c.want)
		}
	}
}

func TestNumericLess(t *testing.T) {
	t.Parallel()
	if !numericLess("2", "10") {
		t.Fatal("numericLess(2,10) should be true (numeric), got false (lexical would be false)")
	}
	if numericLess("10", "2") {
		t.Fatal("numericLess(10,2) should be false")
	}
	// Non-numeric falls back to lexical.
	if !numericLess("apple", "banana") {
		t.Fatal("numericLess non-numeric should fall back to lexical")
	}
	// Equal numeric value ties broken lexically.
	if numericLess("1.0", "1.0") {
		t.Fatal("equal values must not be less")
	}
}

func TestSortedLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(path, []byte("cherry\napple\nbanana\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := sortedLines(path, false, false)
	want := []string{"apple", "banana", "cherry"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// reverse
	rev := sortedLines(path, false, true)
	if rev[0] != "cherry" || rev[2] != "apple" {
		t.Fatalf("reverse sort = %v", rev)
	}
}
