package linediff

import "testing"

// These cases are ported from ayame-editor's diff.rs test suite so the Go port
// keeps behavioral parity with the reference implementation.

func hunkTuple(h Hunk) [5]uint64 {
	return [5]uint64{uint64(h.Kind), h.OldStart, h.OldLen, h.NewStart, h.NewLen}
}

func diffStrings(t *testing.T, oldText, newText string, maxHunks int, window uint64) Result {
	t.Helper()
	return Diff(SplitLines(oldText), SplitLines(newText), maxHunks, window)
}

func TestSplitLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a\nb\nc\n", []string{"a", "b", "c"}},
		{"a\nb", []string{"a", "b"}},
		{"a\r\nb\r\n", []string{"a", "b"}},
		{"a\rb\rc", []string{"a", "b", "c"}},
		{"a\rb\nc\r\nd", []string{"a", "b", "c", "d"}},
		{"a\n\n", []string{"a", ""}},
		{"\n", []string{""}},
	}
	for _, c := range cases {
		got := SplitLines(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("SplitLines(%q) len = %d, want %d (%q)", c.in, len(got), len(c.want), got)
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Fatalf("SplitLines(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestSplitTextLinesPreservesTerminators(t *testing.T) {
	t.Parallel()
	lines := SplitTextLines("lf\ncrlf\r\ncr\rfinal")
	if lines.Count() != 4 {
		t.Fatalf("count = %d", lines.Count())
	}
	for i, want := range []string{"\n", "\r\n", "\r", ""} {
		if got := lines.LineEnding(uint64(i)); got != want {
			t.Errorf("ending %d = %q, want %q", i, got, want)
		}
	}
}

func TestCROnlyDiffIsLineBased(t *testing.T) {
	t.Parallel()
	result := diffStrings(t, "a\rb\rc", "a\rb\rX", 200, 128)
	if result.OldLines != 3 || result.NewLines != 3 || result.Modified != 1 || result.HunkCount != 1 {
		t.Fatalf("CR-only diff = %+v", result)
	}
	if hunk := result.Hunks[0]; hunk.OldStart != 2 || hunk.NewStart != 2 || hunk.OldLen != 1 || hunk.NewLen != 1 {
		t.Fatalf("CR-only hunk = %+v", hunk)
	}
}

func TestEqualDocumentsProduceNoHunks(t *testing.T) {
	t.Parallel()
	res := diffStrings(t, "a\nb\nc\n", "a\nb\nc\n", 200, 128)
	if len(res.Hunks) != 0 || res.HunkCount != 0 {
		t.Fatalf("hunks = %v, count = %d", res.Hunks, res.HunkCount)
	}
	if res.Added != 0 || res.Deleted != 0 || res.Modified != 0 {
		t.Fatalf("stats = (%d,%d,%d)", res.Added, res.Deleted, res.Modified)
	}
	if res.OldLines != 3 || res.NewLines != 3 {
		t.Fatalf("lines = (%d,%d)", res.OldLines, res.NewLines)
	}
}

func TestChangedLineIsReplace(t *testing.T) {
	t.Parallel()
	res := diffStrings(t, "a\nb\nc\n", "a\nX\nc\n", 200, 128)
	if res.HunkCount != 1 {
		t.Fatalf("hunk_count = %d", res.HunkCount)
	}
	if got := hunkTuple(res.Hunks[0]); got != [5]uint64{uint64(Replace), 1, 1, 1, 1} {
		t.Fatalf("hunk = %v", got)
	}
	if res.Added != 0 || res.Deleted != 0 || res.Modified != 1 {
		t.Fatalf("stats = (%d,%d,%d)", res.Added, res.Deleted, res.Modified)
	}
}

func TestInsertedLinesMakeInsertHunk(t *testing.T) {
	t.Parallel()
	res := diffStrings(t, "a\nb\n", "a\nx\ny\nb\n", 200, 128)
	if res.HunkCount != 1 {
		t.Fatalf("hunk_count = %d", res.HunkCount)
	}
	if got := hunkTuple(res.Hunks[0]); got != [5]uint64{uint64(Insert), 1, 0, 1, 2} {
		t.Fatalf("hunk = %v", got)
	}
	if res.Added != 2 || res.Deleted != 0 || res.Modified != 0 {
		t.Fatalf("stats = (%d,%d,%d)", res.Added, res.Deleted, res.Modified)
	}
}

func TestDeletedLinesMakeDeleteHunk(t *testing.T) {
	t.Parallel()
	res := diffStrings(t, "a\nx\ny\nb\n", "a\nb\n", 200, 128)
	if res.HunkCount != 1 {
		t.Fatalf("hunk_count = %d", res.HunkCount)
	}
	if got := hunkTuple(res.Hunks[0]); got != [5]uint64{uint64(Delete), 1, 2, 1, 0} {
		t.Fatalf("hunk = %v", got)
	}
	if res.Added != 0 || res.Deleted != 2 || res.Modified != 0 {
		t.Fatalf("stats = (%d,%d,%d)", res.Added, res.Deleted, res.Modified)
	}
}

func TestTrailingInsertAndEmptyOldDocument(t *testing.T) {
	t.Parallel()
	res := diffStrings(t, "", "a\nb\n", 200, 128)
	if res.HunkCount != 1 {
		t.Fatalf("hunk_count = %d", res.HunkCount)
	}
	if got := hunkTuple(res.Hunks[0]); got != [5]uint64{uint64(Insert), 0, 0, 0, 2} {
		t.Fatalf("hunk = %v", got)
	}
	if res.Added != 2 {
		t.Fatalf("added = %d", res.Added)
	}
}

func TestResyncWindowBoundsTheLookahead(t *testing.T) {
	t.Parallel()
	// Window large enough to see "b" again: one clean insert hunk.
	res := diffStrings(t, "a\nb\n", "a\nx1\nx2\nx3\nb\n", 200, 3)
	if res.HunkCount != 1 {
		t.Fatalf("window=3 hunk_count = %d", res.HunkCount)
	}
	if got := hunkTuple(res.Hunks[0]); got != [5]uint64{uint64(Insert), 1, 0, 1, 3} {
		t.Fatalf("window=3 hunk = %v", got)
	}
	if res.Added != 3 || res.Deleted != 0 || res.Modified != 0 {
		t.Fatalf("window=3 stats = (%d,%d,%d)", res.Added, res.Deleted, res.Modified)
	}

	// Window too small to resync: a replace, then a trailing insert.
	res = diffStrings(t, "a\nb\n", "a\nx1\nx2\nx3\nb\n", 200, 2)
	if res.HunkCount != 2 {
		t.Fatalf("window=2 hunk_count = %d", res.HunkCount)
	}
	if got := hunkTuple(res.Hunks[0]); got != [5]uint64{uint64(Replace), 1, 1, 1, 1} {
		t.Fatalf("window=2 hunk[0] = %v", got)
	}
	if got := hunkTuple(res.Hunks[1]); got != [5]uint64{uint64(Insert), 2, 0, 2, 3} {
		t.Fatalf("window=2 hunk[1] = %v", got)
	}
	if res.Added != 3 || res.Deleted != 0 || res.Modified != 1 {
		t.Fatalf("window=2 stats = (%d,%d,%d)", res.Added, res.Deleted, res.Modified)
	}
}

func TestMaxHunksTruncatesButKeepsCounting(t *testing.T) {
	t.Parallel()
	res := diffStrings(t, "1\na\n2\nb\n3\nc\n", "1\nA\n2\nB\n3\nC\n", 2, 128)
	if res.HunkCount != 3 {
		t.Fatalf("hunk_count = %d", res.HunkCount)
	}
	if len(res.Hunks) != 2 {
		t.Fatalf("stored hunks = %d", len(res.Hunks))
	}
	if res.OmittedHunks != 1 {
		t.Fatalf("omitted = %d", res.OmittedHunks)
	}
	if res.Modified != 3 {
		t.Fatalf("modified = %d", res.Modified)
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()
	if Insert.String() != "insert" || Delete.String() != "delete" || Replace.String() != "replace" {
		t.Fatalf("kind strings: %s %s %s", Insert, Delete, Replace)
	}
}
