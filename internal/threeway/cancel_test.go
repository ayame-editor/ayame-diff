package threeway

import (
	"context"
	"errors"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/linediff"
)

// TestCompareContextCancels covers #169: a cancelled context aborts the two
// underlying two-way diffs of a three-way comparison.
func TestCompareContextCancels(t *testing.T) {
	t.Parallel()
	lines := make(linediff.StringLines, 5000)
	for i := range lines {
		lines[i] = "same"
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CompareContext(ctx, lines, lines, lines, linediff.Options{Window: 128}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled three-way compare: err = %v, want context.Canceled", err)
	}
}
