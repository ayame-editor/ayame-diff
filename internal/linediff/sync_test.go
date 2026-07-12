package linediff

import "testing"

func TestSyncPointRecoversBeyondWindow(t *testing.T) {
	t.Parallel()
	old := SplitLines("start\na\nb\nc\nANCHOR\ntail\n")
	new := SplitLines("start\nx1\nx2\nx3\nx4\nx5\na\nb\nc\nANCHOR\ntail\n")
	without := mustDiffWith(t, old, new, Options{MaxHunks: 100, Window: 2})
	with := mustDiffWith(t, old, new, Options{
		MaxHunks: 100, Window: 2, SyncPoints: []SyncPoint{{Old: 1, New: 6}},
	})
	if with.Modified >= without.Modified || with.Added != 5 {
		t.Fatalf("sync did not recover alignment: without=%+v with=%+v", without, with)
	}
}

func TestDiffWithRejectsInvalidSyncPoints(t *testing.T) {
	t.Parallel()
	old := SplitLines("a\nb\n")
	new := SplitLines("a\nc\n")
	for _, points := range [][]SyncPoint{
		{{Old: 2, New: 1}},
		{{Old: 1, New: 1}, {Old: 0, New: 1}},
	} {
		if _, err := DiffWith(old, new, Options{SyncPoints: points}); err == nil {
			t.Fatalf("DiffWith silently accepted invalid points: %+v", points)
		}
	}
}

func TestValidateSyncPoints(t *testing.T) {
	t.Parallel()
	if err := ValidateSyncPoints([]SyncPoint{{Old: 1, New: 2}, {Old: 3, New: 4}}, 5, 6); err != nil {
		t.Fatal(err)
	}
	for _, points := range [][]SyncPoint{
		{{Old: 5, New: 1}},
		{{Old: 2, New: 2}, {Old: 1, New: 3}},
		{{Old: 1, New: 3}, {Old: 2, New: 2}},
	} {
		if err := ValidateSyncPoints(points, 5, 5); err == nil {
			t.Fatalf("invalid points accepted: %+v", points)
		}
	}
}

func TestIgnoreHunksTracksAndFoldsSelection(t *testing.T) {
	t.Parallel()
	old := SplitLines("a\nb\nc\n")
	new := SplitLines("a\nB\nc\nd\n")
	result := Diff(old, new, 100, 128)
	IgnoreHunks(&result, []int{0})
	if result.IgnoredHunks != 1 || len(result.Hunks) != 1 || result.Modified != 0 || result.Added != 1 {
		t.Fatalf("ignored result = %+v", result)
	}
}
