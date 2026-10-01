package linediff

import "testing"

// TestAlignWhitespaceKeepsReindentedPositions is the #270 case: a comparison
// that ignores whitespace only for alignment must still report the whitespace
// difference, at the original position, and must not let the re-indented lines
// vanish.
func TestAlignWhitespaceKeepsReindentedPositions(t *testing.T) {
	t.Parallel()
	old := SplitLines("  a\n  b\n  c\n")
	new := SplitLines("a\nb\nc\n")

	// Ignoring whitespace for comparison hides the whole re-indentation.
	if r := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, Whitespace: WSChange}); r.HunkCount != 0 {
		t.Fatalf("ignore-for-diff hunks = %d, want 0 (%+v)", r.HunkCount, r.Hunks)
	}

	// Ignoring whitespace only for alignment keeps one hunk covering the
	// re-indented block, with both sides aligned line for line.
	r := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, AlignWhitespace: WSChange})
	if r.HunkCount != 1 || len(r.Hunks) != 1 {
		t.Fatalf("align hunks = %d, want 1 (%+v)", r.HunkCount, r.Hunks)
	}
	h := r.Hunks[0]
	if h.Kind != Replace || h.OldStart != 0 || h.OldLen != 3 || h.NewStart != 0 || h.NewLen != 3 {
		t.Fatalf("align hunk = %+v", h)
	}
	if r.Modified != 3 || r.Added != 0 || r.Deleted != 0 {
		t.Fatalf("align stats = added %d deleted %d modified %d", r.Added, r.Deleted, r.Modified)
	}
}

// TestAlignWhitespacePreservesAContentDifference checks the whole point of the
// split: the real change is still reported while the surrounding re-indentation
// stays aligned.
func TestAlignWhitespacePreservesAContentDifference(t *testing.T) {
	t.Parallel()
	old := SplitLines("  alpha\n  beta\n  gamma\n")
	new := SplitLines("alpha\nbeta\nGAMMA\n")

	r := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, AlignWhitespace: WSChange})
	if r.HunkCount != 2 || len(r.Hunks) != 2 {
		t.Fatalf("hunks = %d, want 2 (%+v)", r.HunkCount, r.Hunks)
	}
	// The re-indented prefix coalesces into one aligned replace...
	if h := r.Hunks[0]; h.OldStart != 0 || h.OldLen != 2 || h.NewStart != 0 || h.NewLen != 2 {
		t.Fatalf("aligned prefix = %+v", h)
	}
	// ...and the content change keeps its own aligned position.
	if h := r.Hunks[1]; h.OldStart != 2 || h.OldLen != 1 || h.NewStart != 2 || h.NewLen != 1 {
		t.Fatalf("content hunk = %+v", h)
	}
	if r.Modified != 3 || r.Added != 0 || r.Deleted != 0 {
		t.Fatalf("stats = added %d deleted %d modified %d", r.Added, r.Deleted, r.Modified)
	}
}

// TestAlignWhitespaceRestoresCorrespondence is the misalignment the feature
// fixes: an inserted line among re-indented ones used to shift every later line
// because the exact comparison could not find an anchor.
func TestAlignWhitespaceRestoresCorrespondence(t *testing.T) {
	t.Parallel()
	old := SplitLines("a\nb\nc\n")
	new := SplitLines("  a\nx\n  b\n  c\n")

	r := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, AlignWhitespace: WSChange})
	if r.HunkCount != 3 {
		t.Fatalf("hunk_count = %d, want 3 (%+v)", r.HunkCount, r.Hunks)
	}
	// a is a re-indented replace, x is inserted, and b/c stay matched.
	if h := r.Hunks[0]; h.Kind != Replace || h.OldStart != 0 || h.OldLen != 1 || h.NewStart != 0 || h.NewLen != 1 {
		t.Fatalf("hunk 0 = %+v", h)
	}
	if h := r.Hunks[1]; h.Kind != Insert || h.OldStart != 1 || h.OldLen != 0 || h.NewStart != 1 || h.NewLen != 1 {
		t.Fatalf("hunk 1 = %+v", h)
	}
	if h := r.Hunks[2]; h.Kind != Replace || h.OldStart != 1 || h.OldLen != 2 || h.NewStart != 2 || h.NewLen != 2 {
		t.Fatalf("hunk 2 = %+v", h)
	}
	if r.Added != 1 || r.Modified != 3 || r.Deleted != 0 {
		t.Fatalf("stats = added %d deleted %d modified %d", r.Added, r.Deleted, r.Modified)
	}
}

// TestAlignWhitespaceDoesNotHideCase confirms alignment is only a matching aid:
// the comparison policy still decides what counts as a difference.
func TestAlignWhitespaceDoesNotHideCase(t *testing.T) {
	t.Parallel()
	old := SplitLines("  Foo\n")
	new := SplitLines("foo\n")

	r := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, IgnoreCase: true, AlignWhitespace: WSChange})
	if r.HunkCount != 1 || r.Hunks[0].OldStart != 0 || r.Hunks[0].NewStart != 0 {
		t.Fatalf("hunks = %+v", r.Hunks)
	}
	// With case ignored for comparison too, the same pair is unchanged.
	r = mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, IgnoreCase: true, Whitespace: WSChange})
	if r.HunkCount != 0 {
		t.Fatalf("ignore-for-diff hunks = %d, want 0", r.HunkCount)
	}
}

// TestAlignWhitespaceDoesNotFoldEOLChanges pins that an EOL-only change is
// still reported. Alignment matches lines, not terminators, but it must not turn
// a CRLF/LF change into nothing.
func TestAlignWhitespaceDoesNotFoldEOLChanges(t *testing.T) {
	t.Parallel()
	old := SplitTextLines("a\n")
	new := SplitTextLines("a\r\n")
	r := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, AlignWhitespace: WSChange})
	if r.HunkCount != 1 || r.Modified != 1 {
		t.Fatalf("EOL with alignment = %+v", r)
	}
}

// TestAlignWhitespaceZeroValueIsUnchanged guards the default: leaving the field
// alone must reproduce the exact pre-#270 comparison, whitespace-only lines
// vanishing under an ignore policy.
func TestAlignWhitespaceZeroValueIsUnchanged(t *testing.T) {
	t.Parallel()
	old := SplitLines("  a\n  b\n")
	new := SplitLines("a\nb\n")

	withDefault := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, Whitespace: WSChange})
	explicit := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, Whitespace: WSChange, AlignWhitespace: WSKeep})
	if withDefault.HunkCount != explicit.HunkCount || withDefault.Modified != explicit.Modified {
		t.Fatalf("zero-value alignment changed the result: %+v vs %+v", withDefault, explicit)
	}
	if withDefault.HunkCount != 0 {
		t.Fatalf("hunk_count = %d, want 0", withDefault.HunkCount)
	}
}

// TestAlignWhitespaceNotCoarserIsNoOp pins that a redundant alignment policy
// cannot change the diff: when comparison already ignores at least as much
// whitespace, alignment is exactly comparison.
func TestAlignWhitespaceNotCoarserIsNoOp(t *testing.T) {
	t.Parallel()
	old := SplitLines("a b c\n  x  y  \n")
	new := SplitLines("abc\nx y\n")

	compare := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, Whitespace: WSAll})
	redundant := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 128, Whitespace: WSAll, AlignWhitespace: WSChange})
	if compare.HunkCount != redundant.HunkCount || compare.Modified != redundant.Modified {
		t.Fatalf("redundant alignment changed the result: %+v vs %+v", compare, redundant)
	}
	if compare.HunkCount != 0 {
		t.Fatalf("hunk_count = %d, want 0", compare.HunkCount)
	}
}

// TestAlignWhitespaceKeepsStreamingWindowBound checks the alignment view reuses
// the same bounded resync window: a difference farther away than the window is
// not silently matched.
func TestAlignWhitespaceKeepsStreamingWindowBound(t *testing.T) {
	t.Parallel()
	old := SplitLines("  a\n  b\n  c\n  d\n  e\n")
	new := SplitLines("a\nb\nc\nX\nd\ne\n")
	r := mustDiffWith(t, old, new, Options{MaxHunks: 200, Window: 1, AlignWhitespace: WSChange})
	// Window 1 cannot look past the insertion, so lines after it resync as
	// replacements rather than staying silent.
	if r.HunkCount == 0 {
		t.Fatalf("window-bounded align diff = %+v", r)
	}
	if r.OldLines != 5 || r.NewLines != 6 {
		t.Fatalf("lines = %d/%d", r.OldLines, r.NewLines)
	}
}
