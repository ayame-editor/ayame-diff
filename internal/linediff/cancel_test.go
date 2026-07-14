package linediff

import (
	"context"
	"errors"
	"testing"
)

// TestDiffWithContextCancels covers #169: a cancelled context aborts the diff
// walk early instead of running to completion.
func TestDiffWithContextCancels(t *testing.T) {
	t.Parallel()
	lines := make(StringLines, 4*cancelCheckInterval)
	for i := range lines {
		lines[i] = "same line"
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DiffWithContext(ctx, lines, lines, Options{MaxHunks: 100, Window: 128}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled diff: err = %v, want context.Canceled", err)
	}
	// Without cancellation the same inputs complete cleanly.
	if _, err := DiffWithContext(context.Background(), lines, lines, Options{MaxHunks: 100, Window: 128}); err != nil {
		t.Fatalf("uncancelled diff: %v", err)
	}
}

// TestDetectMovesContextCancels covers #169 for the move-detection pass.
func TestDetectMovesContextCancels(t *testing.T) {
	t.Parallel()
	res := &Result{Hunks: []Hunk{
		{Kind: Delete, OldStart: 0, OldLen: 3},
		{Kind: Insert, NewStart: 0, NewLen: 3},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	old := StringLines{"a", "b", "c"}
	if _, err := DetectMovesContext(ctx, old, old, res, MoveOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled move detection: err = %v, want context.Canceled", err)
	}
}
