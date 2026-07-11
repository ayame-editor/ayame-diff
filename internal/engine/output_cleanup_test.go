package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAssembleOutputCleansTemporaryFileOnCancellation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	part := filepath.Join(dir, "part.tsv")
	if err := os.WriteFile(part, []byte("row\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := assembleOutput(ctx, filepath.Join(dir, "result.tsv"), []string{part}, nil, false, false, "tsv")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("assembleOutput error = %v", err)
	}
	temps, err := filepath.Glob(filepath.Join(dir, ".ayame-diff-output-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary outputs remain: %v", temps)
	}
}
