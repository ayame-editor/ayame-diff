package dircompare

import (
	"context"
	"errors"
	"testing"
)

// TestCompareContextCancels covers #169: a cancelled context aborts the (up to
// 64-worker) directory content comparison instead of running to completion.
func TestCompareContextCancels(t *testing.T) {
	t.Parallel()
	oldDir, newDir := t.TempDir(), t.TempDir()
	write(t, oldDir, "a.txt", "alpha")
	write(t, newDir, "a.txt", "beta")
	write(t, oldDir, "b.txt", "gamma")
	write(t, newDir, "b.txt", "delta")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CompareContext(ctx, oldDir, newDir, Options{Workers: 4}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled compare: err = %v, want context.Canceled", err)
	}
	// The same inputs complete without cancellation.
	if _, err := CompareContext(context.Background(), oldDir, newDir, Options{Workers: 4}); err != nil {
		t.Fatalf("uncancelled compare: %v", err)
	}
}
