package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// spillBudgetConfig uses the smallest budget the validator allows for one
// worker. A single worker then has a 12MiB sort chunk (16MiB * 3/4), so a
// partition wider than that necessarily builds more than one run — the engine's
// definition of spilling to storage. This makes offload deterministic instead of
// depending on machine memory (#138).
func spillBudgetConfig(left, right, out, work string) Config {
	cfg := testConfig(left, right, out)
	cfg.Partitions = 2
	cfg.ParseWorkers = 1
	cfg.Workers = 1
	cfg.MemoryText = "16MiB"
	cfg.PartitionBufferText = "16KiB"
	cfg.WorkDir = work
	return cfg
}

// writeWideTSV writes a two-column TSV whose rows are each about payload bytes,
// so a small row count still exceeds the sort chunk.
func writeWideTSV(t *testing.T, path string, rows, payload int) {
	t.Helper()
	var b strings.Builder
	b.WriteString("id\tvalue\n")
	filler := strings.Repeat("x", payload)
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&b, "%d\t%s\n", i, filler)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertWorkRootEmpty(t *testing.T, work string) {
	t.Helper()
	entries, err := os.ReadDir(work)
	if err != nil {
		t.Fatalf("explicit work root was removed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("work root was not cleaned: %v", entries)
	}
}

// TestMakeSortedFileReportsSpill pins the spill signal itself: one run that fits
// the chunk is not a spill, several runs are.
func TestMakeSortedFileReportsSpill(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	partitionPath := filepath.Join(dir, "part.bin")
	f, err := os.Create(partitionPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(f)
	for i := 0; i < 200; i++ {
		key, row, err := encodeStringFields([]string{fmt.Sprintf("key-%04d", i), strings.Repeat("v", 128)}, []int{0, 1}, []int{0}, false, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeBinRecord(writer, binRecord{Key: append([]byte(nil), key...), Row: append([]byte(nil), row...)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	spilledPath, spilled, err := makeSortedFile(context.Background(), partitionPath, filepath.Join(dir, "small"), "small", 512, 3, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if !spilled {
		t.Fatalf("a %q chunk should spill; path=%s", "512B", spilledPath)
	}
	residentPath, spilled, err := makeSortedFile(context.Background(), partitionPath, filepath.Join(dir, "large"), "large", 1<<20, 3, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if spilled {
		t.Fatalf("a 1MiB chunk should hold every run in memory; path=%s", residentPath)
	}
}

// TestRunReportsSpillAndCleansWorkRoot covers #138's reporting requirement and
// its cleanup requirement on the success path: the summary names the budget it
// ran under, says it spilled, and leaves the user work root empty.
func TestRunReportsSpillAndCleansWorkRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "out.tsv")
	work := filepath.Join(dir, "work")
	writeWideTSV(t, leftPath, 600, 64*1024)
	writeWideTSV(t, rightPath, 600, 64*1024)

	cfg := spillBudgetConfig(leftPath, rightPath, outPath, work)
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Spilled {
		t.Fatalf("summary.Spilled = false for a partition larger than the budget: %#v", summary)
	}
	if summary.MemoryBudgetBytes != 16<<20 {
		t.Fatalf("MemoryBudgetBytes = %d, want %d", summary.MemoryBudgetBytes, 16<<20)
	}
	assertWorkRootEmpty(t, work)
}

// TestRunCancellationDuringSpillReturnsPromptly is the responsiveness guard: a
// comparison that has started spilling must still honour cancellation quickly and
// leave no temporary files behind.
func TestRunCancellationDuringSpillReturnsPromptly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "out.tsv")
	work := filepath.Join(dir, "work")
	writeWideTSV(t, leftPath, 1000, 64*1024)
	writeWideTSV(t, rightPath, 1000, 64*1024)

	cfg := spillBudgetConfig(leftPath, rightPath, outPath, work)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := Run(ctx, cfg)
		result <- err
	}()

	waitForSpillRunFile(t, work, 30*time.Second)
	started := time.Now()
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
		if elapsed := time.Since(started); elapsed > 5*time.Second {
			t.Fatalf("cancellation during spill took %s, want under 5s", elapsed)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return promptly after cancellation during spill")
	}
	assertWorkRootEmpty(t, work)
}

// TestRunCleansWorkRootOnFailure checks the failure path: a parse error after the
// partition files were created still removes the generated contents.
func TestRunCleansWorkRootOnFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "out.tsv")
	work := filepath.Join(dir, "work")
	mustWriteFile(t, leftPath, "id\tvalue\n1\ta\nbroken\n")
	mustWriteFile(t, rightPath, "id\tvalue\n1\ta\n")

	cfg := spillBudgetConfig(leftPath, rightPath, outPath, work)
	if _, err := Run(context.Background(), cfg); err == nil {
		t.Fatal("Run accepted a record with the wrong column count")
	}
	assertWorkRootEmpty(t, work)
}

// waitForSpillRunFile waits until the external sort has written a run file under
// root, which means offload is underway rather than merely scheduled.
func waitForSpillRunFile(t *testing.T, root string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		found := false
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".bin") && strings.Contains(entry.Name(), "-run-") {
				found = true
				return fs.SkipAll
			}
			return nil
		})
		if found {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("comparison did not start spilling under %s within %s", root, timeout)
}
