package linediff

import (
	"fmt"
	"strings"
	"testing"
)

func TestDetectMovesPairsExactDeleteAndInsert(t *testing.T) {
	t.Parallel()
	old := SplitLines("top\nmove-a\nmove-b\nstay-a\nstay-b\nstay-c\nbottom\n")
	new := SplitLines("top\nstay-a\nstay-b\nstay-c\nmove-a\nmove-b\nbottom\n")
	res := Diff(old, new, 100, 128)
	if pairs := DetectMoves(old, new, &res, MoveOptions{MinLines: 2, MaxCandidates: 100}); pairs != 1 {
		t.Fatalf("pairs = %d; hunks=%+v", pairs, res.Hunks)
	}
	var deleteHunk, insertHunk *Hunk
	for i := range res.Hunks {
		switch res.Hunks[i].Kind {
		case Delete:
			deleteHunk = &res.Hunks[i]
		case Insert:
			insertHunk = &res.Hunks[i]
		}
	}
	if deleteHunk == nil || insertHunk == nil || deleteHunk.MoveID == 0 || deleteHunk.MoveID != insertHunk.MoveID {
		t.Fatalf("move annotations = delete %+v insert %+v", deleteHunk, insertHunk)
	}
	if res.MovedBlocks != 1 || res.MovedLines != 2 {
		t.Fatalf("move totals = %d/%d", res.MovedBlocks, res.MovedLines)
	}
	if res.Added != 0 || res.Deleted != 0 {
		t.Fatalf("moves should not remain false added/deleted stats: %d/%d", res.Added, res.Deleted)
	}
}

func TestDetectMovesHonorsMinimumAndOmittedGuard(t *testing.T) {
	t.Parallel()
	old := SplitLines("a\nx\nb\n")
	new := SplitLines("a\nb\nx\n")
	res := Diff(old, new, 100, 128)
	if DetectMoves(old, new, &res, MoveOptions{MinLines: 2}) != 0 {
		t.Fatal("one-line block should be below the minimum")
	}
	res.OmittedHunks = 1
	if DetectMoves(old, new, &res, MoveOptions{MinLines: 1}) != 0 {
		t.Fatal("truncated hunk sets must not produce partial move pairs")
	}
}

func BenchmarkDiffMoveDetection(b *testing.B) {
	var moved, stayed strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&moved, "moved-%04d\n", i)
	}
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&stayed, "stayed-%04d\n", i)
	}
	old := SplitLines(moved.String() + stayed.String())
	new := SplitLines(stayed.String() + moved.String())
	b.Run("off", func(b *testing.B) {
		for range b.N {
			_ = Diff(old, new, 10_000, 1024)
		}
	})
	b.Run("on", func(b *testing.B) {
		for range b.N {
			res := Diff(old, new, 10_000, 1024)
			DetectMoves(old, new, &res, MoveOptions{MinLines: 2, MaxCandidates: 10_000})
		}
	})
}
