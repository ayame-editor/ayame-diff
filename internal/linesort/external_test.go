package linesort

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

// assertLines checks a sorted Result line by line.
func assertLines(t *testing.T, got linediff.Lines, want []string) {
	t.Helper()
	if int(got.Count()) != len(want) {
		t.Fatalf("count=%d want %d", got.Count(), len(want))
	}
	for i, expected := range want {
		line, ok := got.Line(uint64(i))
		if !ok {
			t.Fatalf("line %d missing", i)
		}
		if line != expected {
			t.Fatalf("line[%d]=%q want %q", i, line, expected)
		}
	}
}

// TestSortSourceSpillsMatchInMemoryResult is the #7 / #137 regression: a budget
// too small to hold the input must still produce exactly the order an unbounded
// in-memory sort would, by spilling runs and merging them.
func TestSortSourceSpillsMatchInMemoryResult(t *testing.T) {
	t.Parallel()
	random := rand.New(rand.NewSource(1))
	lines := make([]string, 4000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%06d-%s", random.Intn(100000), strings.Repeat("x", random.Intn(40)))
	}
	source := linediff.StringLines(append([]string(nil), lines...))

	want := SortLines(lines, false, false)

	// A budget of a few hundred bytes forces many runs and several merge
	// passes, exercising the bounded fan-in rather than a single k-way merge.
	got, err := SortSource(source, Options{MemoryBytes: 500, TempDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	if !got.Spilled() {
		t.Fatal("a budget far below the input did not spill")
	}
	assertLines(t, got, []string(want))
}

// TestSortSourceStaysInMemoryWithinBudget keeps the common case off disk.
func TestSortSourceStaysInMemoryWithinBudget(t *testing.T) {
	t.Parallel()
	source := linediff.StringLines([]string{"c", "a", "b"})
	got, err := SortSource(source, Options{MemoryBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	if got.Spilled() {
		t.Error("a tiny input spilled to disk")
	}
	assertLines(t, got, []string{"a", "b", "c"})
}

// TestSortSourceCloseRemovesSpillFiles keeps a long-running GUI server from
// leaking temp files across comparisons.
func TestSortSourceCloseRemovesSpillFiles(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = strconv.Itoa(500 - i)
	}
	got, err := SortSource(linediff.StringLines(lines), Options{MemoryBytes: 200, TempDir: temp})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Spilled() {
		t.Fatal("expected a spill")
	}
	entries, err := os.ReadDir(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no spill directory was created")
	}
	if err := got.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	entries, err = os.ReadDir(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("spill files survived Close: %v", entries)
	}
	// Close must be safe to call again (defer plus explicit call).
	if err := got.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestSortSourceSpillPreservesEveryLine guards against the merge dropping or
// duplicating data, which a sort must never do.
func TestSortSourceSpillPreservesEveryLine(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := range 1000 {
		// Deliberate duplicates: a multiset must survive the merge intact.
		lines = append(lines, strconv.Itoa(i%250))
	}
	got, err := SortSource(linediff.StringLines(append([]string(nil), lines...)), Options{MemoryBytes: 300, TempDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	if int(got.Count()) != len(lines) {
		t.Fatalf("count=%d want %d — the merge lost or duplicated lines", got.Count(), len(lines))
	}
	seen := make([]string, 0, len(lines))
	for i := uint64(0); i < got.Count(); i++ {
		line, _ := got.Line(i)
		seen = append(seen, line)
	}
	if !sort.SliceIsSorted(seen, func(a, b int) bool { return seen[a] < seen[b] }) {
		t.Error("merged output is not sorted")
	}
	sort.Strings(lines)
	for i := range lines {
		if seen[i] != lines[i] {
			t.Fatalf("multiset differs at %d: %q vs %q", i, seen[i], lines[i])
		}
	}
}

// TestSortSourceSpillHonorsNumericAndReverse checks the ordering options reach
// the run writer and the merge identically.
func TestSortSourceSpillHonorsNumericAndReverse(t *testing.T) {
	t.Parallel()
	lines := make([]string, 400)
	for i := range lines {
		lines[i] = strconv.Itoa((i * 7919) % 1000)
	}
	for _, tc := range []struct{ numeric, reverse bool }{
		{true, false}, {true, true}, {false, true},
	} {
		name := fmt.Sprintf("numeric=%v/reverse=%v", tc.numeric, tc.reverse)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := SortLines(lines, tc.numeric, tc.reverse)
			got, err := SortSource(linediff.StringLines(append([]string(nil), lines...)),
				Options{Numeric: tc.numeric, Reverse: tc.reverse, MemoryBytes: 250, TempDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer got.Close()
			if !got.Spilled() {
				t.Fatal("expected a spill")
			}
			assertLines(t, got, []string(want))
		})
	}
}

// TestSortedWithOptionsSpillsFromAFile drives the file entry point end to end,
// including the encoding-aware read the CLI and server go through.
func TestSortedWithOptionsSpillsFromAFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "in.txt")
	var builder strings.Builder
	for i := range 2000 {
		fmt.Fprintf(&builder, "%06d\n", (i*31)%2000)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := SortedWithOptions(path, "auto", Options{MemoryBytes: 400, TempDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	if !got.Spilled() {
		t.Fatal("expected a spill")
	}
	if got.Count() != 2000 {
		t.Fatalf("count=%d want 2000", got.Count())
	}
	for i := uint64(0); i < got.Count(); i++ {
		line, _ := got.Line(i)
		if line != fmt.Sprintf("%06d", i) {
			t.Fatalf("line[%d]=%q want %06d", i, line, i)
		}
	}
}

// TestSortSourceSpillKeepsEmptyLines checks the newline framing round-trips an
// empty line rather than swallowing it.
func TestSortSourceSpillKeepsEmptyLines(t *testing.T) {
	t.Parallel()
	lines := []string{"b", "", "a", "", "c"}
	for range 200 {
		lines = append(lines, "", "z")
	}
	want := SortLines(lines, false, false)
	got, err := SortSource(linediff.StringLines(append([]string(nil), lines...)), Options{MemoryBytes: 120, TempDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	if !got.Spilled() {
		t.Fatal("expected a spill")
	}
	assertLines(t, got, []string(want))
}
