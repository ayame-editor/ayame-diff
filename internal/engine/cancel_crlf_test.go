package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCRLFMatchesLF(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	if err := os.WriteFile(leftPath, []byte("id\tvalue\r\n1\ta\r\n2\tb\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, []byte("id\tvalue\n1\ta\n2\tb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(leftPath, rightPath, filepath.Join(dir, "out.tsv"))
	cfg.LeftParser = "simple"
	cfg.RightParser = "simple"
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 2 || summary.DiffRows != 0 {
		t.Fatalf("CRLF created false differences: %#v", summary)
	}
}

func TestRunCancellationCleansWorkRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	content := "id\tvalue\n" + strings.Repeat("1\tvalue\n", 500_000)
	if err := os.WriteFile(leftPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	workRoot := filepath.Join(dir, "work")
	cfg := testConfig(leftPath, rightPath, filepath.Join(dir, "out.tsv"))
	cfg.WorkDir = workRoot
	cfg.ParseWorkers = 4

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := Run(ctx, cfg)
		result <- err
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		entries, err := os.ReadDir(workRoot)
		if err == nil && len(entries) > 0 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("Run did not begin work before timeout")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return promptly after cancellation")
	}
	entries, err := os.ReadDir(workRoot)
	if err != nil {
		t.Fatalf("explicit work root was removed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("work root was not cleaned: %v", entries)
	}
}
