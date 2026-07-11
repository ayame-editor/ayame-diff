package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestPreferRootCause(t *testing.T) {
	t.Parallel()
	root := errors.New("disk full")
	wrappedCancel := fmt.Errorf("process partition 3: %w", context.Canceled)
	wrappedRoot := fmt.Errorf("process partition 1: %w", root)

	// A real error must replace a previously-recorded cancellation, and never
	// the other way round — so the reported error names the root cause (#40).
	if got := preferRootCause(nil, wrappedRoot); got != wrappedRoot {
		t.Fatalf("nil + root => %v", got)
	}
	if got := preferRootCause(wrappedCancel, wrappedRoot); got != wrappedRoot {
		t.Fatalf("cancel + root => %v, want the root cause", got)
	}
	if got := preferRootCause(wrappedRoot, wrappedCancel); got != wrappedRoot {
		t.Fatalf("root + cancel => %v, want the root cause preserved", got)
	}
	// First real error wins over a later different real error.
	other := fmt.Errorf("process partition 2: %w", errors.New("bad record"))
	if got := preferRootCause(wrappedRoot, other); got != wrappedRoot {
		t.Fatalf("root + other => %v, want the first real error", got)
	}
	// A bare context.Canceled is still treated as a cancellation.
	if got := preferRootCause(context.Canceled, wrappedRoot); got != wrappedRoot {
		t.Fatalf("bare cancel + root => %v", got)
	}
}
