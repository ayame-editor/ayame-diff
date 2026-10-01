package linediff

import "testing"

// TestMarkDowngradedSurfacesWhitespaceOnlyPair covers #269: with whitespace
// ignored, a line pair that differs only by whitespace must stay visible as a
// downgraded hunk instead of vanishing, and must not count as a difference.
func TestMarkDowngradedSurfacesWhitespaceOnlyPair(t *testing.T) {
	t.Parallel()
	old := SplitLines("alpha\nbeta\n")
	new := SplitLines("alpha \n beta\n")

	// The ordinary ignored comparison drops the pair entirely.
	plain := mustDiffWith(t, old, new, Options{MaxHunks: 50, Window: 8, Whitespace: WSChange})
	if plain.HunkCount != 0 || len(plain.Hunks) != 0 || plain.DowngradedHunks != 0 {
		t.Fatalf("plain ignored diff = %+v", plain)
	}

	got := mustDiffWith(t, old, new, Options{MaxHunks: 50, Window: 8, Whitespace: WSChange, MarkDowngraded: true})
	if got.HunkCount != 0 || got.Modified != 0 || got.Added != 0 || got.Deleted != 0 {
		t.Fatalf("downgraded pair leaked into the difference stats: %+v", got)
	}
	if got.DowngradedHunks != 1 || len(got.Hunks) != 1 {
		t.Fatalf("downgraded hunks = %d, stored = %d (%+v)", got.DowngradedHunks, len(got.Hunks), got.Hunks)
	}
	h := got.Hunks[0]
	if !h.Downgraded || h.Kind != Replace {
		t.Fatalf("hunk = %+v, want a downgraded replace", h)
	}
	// Consecutive dismissed pairs collapse into one contiguous region.
	if h.OldStart != 0 || h.OldLen != 2 || h.NewStart != 0 || h.NewLen != 2 {
		t.Fatalf("region = %+v, want lines 0..2 on both sides", h)
	}
	if got.OmittedHunks != 0 {
		t.Fatalf("omitted = %d, want 0", got.OmittedHunks)
	}
}

// TestMarkDowngradedKeepsEssentialAndDismissedSeparate checks a file with both a
// real change and a whitespace-only one: only the real hunk is a difference.
func TestMarkDowngradedKeepsEssentialAndDismissedSeparate(t *testing.T) {
	t.Parallel()
	old := SplitLines("same\nalpha\nkeep\nold\n")
	new := SplitLines("same\nalpha \nkeep\nnew\n")

	got := mustDiffWith(t, old, new, Options{MaxHunks: 50, Window: 8, Whitespace: WSChange, MarkDowngraded: true})
	if got.HunkCount != 1 || got.DowngradedHunks != 1 || len(got.Hunks) != 2 {
		t.Fatalf("hunks = %+v, want one real and one downgraded", got)
	}
	if got.Modified != 1 {
		t.Fatalf("modified = %d, want 1", got.Modified)
	}
	if !got.Hunks[0].Downgraded || got.Hunks[0].OldStart != 1 {
		t.Fatalf("first hunk = %+v, want the downgraded line 1", got.Hunks[0])
	}
	if got.Hunks[1].Downgraded || got.Hunks[1].OldStart != 3 {
		t.Fatalf("second hunk = %+v, want the real line 3", got.Hunks[1])
	}
}

// TestMarkDowngradedCaseOnly mirrors the whitespace case for ignore-case.
func TestMarkDowngradedCaseOnly(t *testing.T) {
	t.Parallel()
	old := SplitLines("Hello\nWorld\n")
	new := SplitLines("hello\nWORLD\n")

	got := mustDiffWith(t, old, new, Options{MaxHunks: 50, Window: 8, IgnoreCase: true, MarkDowngraded: true})
	if got.HunkCount != 0 || got.DowngradedHunks != 1 || len(got.Hunks) != 1 || !got.Hunks[0].Downgraded {
		t.Fatalf("case-only downgrade = %+v", got)
	}
}

// TestMarkDowngradedNeedsWhitespaceOrCase guards that the flag alone does not
// change an exact comparison, and that EOL-only differences are not dismissed —
// they are structural, not whitespace inside a line.
func TestMarkDowngradedNeedsWhitespaceOrCase(t *testing.T) {
	t.Parallel()
	old := SplitLines("one\ntwo\n")
	new := SplitLines("one\ntwo\n")
	got := mustDiffWith(t, old, new, Options{MaxHunks: 50, Window: 8, MarkDowngraded: true})
	if got.HunkCount != 0 || got.DowngradedHunks != 0 {
		t.Fatalf("exact compare with MarkDowngraded = %+v", got)
	}

	lf, crlf := SplitTextLines("one\ntwo\n"), SplitTextLines("one\r\ntwo\r\n")
	eol := mustDiffWith(t, lf, crlf, Options{MaxHunks: 50, Window: 8, IgnoreEOL: true, MarkDowngraded: true})
	if eol.HunkCount != 0 || eol.DowngradedHunks != 0 {
		t.Fatalf("EOL-only difference should stay dismissed, not downgraded: %+v", eol)
	}
}

// TestMarkDowngradedDoesNotClaimFilteredMatches makes sure a match produced by a
// line filter is not relabelled as a whitespace/case downgrade.
func TestMarkDowngradedDoesNotClaimFilteredMatches(t *testing.T) {
	t.Parallel()
	filters, err := CompileLineFilters([]string{`id=\d+`})
	if err != nil {
		t.Fatal(err)
	}
	old := StringLines{"id=1 x"}
	new := StringLines{"id=2 x"}
	got := mustDiffWith(t, old, new, Options{MaxHunks: 50, Window: 8, LineFilters: filters, MarkDowngraded: true})
	if got.HunkCount != 0 || got.DowngradedHunks != 0 || len(got.Hunks) != 0 {
		t.Fatalf("filtered match leaked into the downgrade lane: %+v", got)
	}
}

// TestMarkDowngradedCountsOmittedHunks checks the separate counters stay
// consistent when the max-hunk cap truncates the stored list.
func TestMarkDowngradedCountsOmittedHunks(t *testing.T) {
	t.Parallel()
	old := SplitLines("a\nb\nc\nd\ne\n")
	new := SplitLines("a \nb\nc \nd\ne \n")
	got := mustDiffWith(t, old, new, Options{MaxHunks: 2, Window: 8, Whitespace: WSChange, MarkDowngraded: true})
	// Three dismissed lines, each separated by a kept line -> three regions.
	if got.DowngradedHunks != 3 {
		t.Fatalf("downgraded = %d, want 3 (%+v)", got.DowngradedHunks, got.Hunks)
	}
	if len(got.Hunks) != 2 {
		t.Fatalf("stored = %d, want 2", len(got.Hunks))
	}
	if got.OmittedHunks != 1 {
		t.Fatalf("omitted = %d, want 1", got.OmittedHunks)
	}
}

// TestMarkDowngradedWithSyncPoints exercises the segmented path, where the
// region offsets and counters are merged per interval.
func TestMarkDowngradedWithSyncPoints(t *testing.T) {
	t.Parallel()
	old := SplitLines("head\nA\nmid\nB\ntail\n")
	new := SplitLines("head\nA \nmid\nB\ntail\n")
	got := mustDiffWith(t, old, new, Options{
		MaxHunks: 50, Window: 8, Whitespace: WSChange, MarkDowngraded: true,
		SyncPoints: []SyncPoint{{Old: 2, New: 2}},
	})
	if got.HunkCount != 0 || got.DowngradedHunks != 1 || len(got.Hunks) != 1 {
		t.Fatalf("sync-point downgrade = %+v", got)
	}
	h := got.Hunks[0]
	if !h.Downgraded || h.OldStart != 1 || h.NewStart != 1 {
		t.Fatalf("sync-point hunk = %+v", h)
	}
	if got.OmittedHunks != 0 {
		t.Fatalf("omitted = %d, want 0", got.OmittedHunks)
	}
}

// TestMarkDowngradedKeepsEssentialHunksIdentical guards the index mapping the UI
// relies on: surfacing dismissed hunks must not change or reorder the real ones,
// so a real hunk keeps the same rank in the marked and unmarked results.
func TestMarkDowngradedKeepsEssentialHunksIdentical(t *testing.T) {
	t.Parallel()
	old := SplitLines("same\nalpha\tx\nmid\nold value\n")
	new := SplitLines("same\nalpha x\nmid\nnew value\n")

	opts := Options{MaxHunks: 50, Window: 8, Whitespace: WSChange}
	plain := mustDiffWith(t, old, new, opts)
	opts.MarkDowngraded = true
	marked := mustDiffWith(t, old, new, opts)

	var essential []Hunk
	for _, hunk := range marked.Hunks {
		if !hunk.Downgraded {
			essential = append(essential, hunk)
		}
	}
	if len(essential) != len(plain.Hunks) {
		t.Fatalf("essential hunks changed: plain=%+v marked=%+v", plain.Hunks, essential)
	}
	for i := range essential {
		if essential[i] != plain.Hunks[i] {
			t.Fatalf("essential hunk %d changed: %+v vs %+v", i, essential[i], plain.Hunks[i])
		}
	}
}

// TestMarkDowngradedMovesSkipDismissed guards that move detection never pairs a
// hunk the ignore options dismissed, even if a future walker produced one as a
// Delete/Insert rather than a Replace.
func TestMarkDowngradedMovesSkipDismissed(t *testing.T) {
	t.Parallel()
	old := StringLines{"x", "y"}
	new := StringLines{"x", "y"}
	res := Result{Hunks: []Hunk{
		{Kind: Delete, OldStart: 0, OldLen: 2, NewStart: 0, NewLen: 0, Downgraded: true},
		{Kind: Insert, OldStart: 0, OldLen: 0, NewStart: 0, NewLen: 2},
	}}
	if pairs := DetectMoves(old, new, &res, MoveOptions{MinLines: 2}); pairs != 0 {
		t.Fatalf("downgraded block was paired as a move: pairs=%d result=%+v", pairs, res)
	}
}
