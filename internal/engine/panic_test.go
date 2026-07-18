package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPartitionWorkerPanicBecomesAnError is the #137 regression. A panic on a
// partition worker goroutine cannot be recovered by Run's caller, so before the
// guard it aborted the CLI outright and took the local GUI server down with
// every other in-flight comparison. It must surface as an ordinary Run error.
func TestPartitionWorkerPanicBecomesAnError(t *testing.T) {
	dir := t.TempDir()
	left, right, out := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "out.tsv")
	if err := os.WriteFile(left, []byte("id,v\n1,a\n2,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,v\n1,a\n2,c\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := processPartitionFn
	processPartitionFn = func(context.Context, int, string, string, int, bool, resolvedConfig, string) (partitionStats, string, error) {
		panic("partition worker exploded")
	}
	t.Cleanup(func() { processPartitionFn = original })

	_, err := Run(context.Background(), panicTestConfig(left, right, out))
	if err == nil {
		t.Fatal("a panicking partition worker did not fail the run")
	}
	if !strings.Contains(err.Error(), "partition worker exploded") {
		t.Fatalf("err=%v must name the panic so the bug stays reportable", err)
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Fatalf("err=%v must be marked as an internal error", err)
	}
}

// TestRunSurvivesRepeatedWorkerPanics proves the process stays usable: a second
// comparison after a panicking one still succeeds.
func TestRunSurvivesRepeatedWorkerPanics(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,v\n1,a\n2,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,v\n1,a\n2,c\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := processPartitionFn
	processPartitionFn = func(context.Context, int, string, string, int, bool, resolvedConfig, string) (partitionStats, string, error) {
		panic("boom")
	}
	if _, err := Run(context.Background(), panicTestConfig(left, right, filepath.Join(dir, "a.tsv"))); err == nil {
		t.Fatal("expected the panicking run to fail")
	}
	processPartitionFn = original

	summary, err := Run(context.Background(), panicTestConfig(left, right, filepath.Join(dir, "b.tsv")))
	if err != nil {
		t.Fatalf("the run after a panic failed: %v", err)
	}
	if summary.DiffRows == 0 {
		t.Error("the recovered process produced no comparison result")
	}
}

func panicTestConfig(left, right, out string) Config {
	return Config{
		LeftPath: left, RightPath: right, OutputPath: out,
		HasHeader: true, AlignColumnsByName: true,
		LeftFormat: "auto", RightFormat: "auto", LeftParser: "auto", RightParser: "auto",
		KeyNames:   []string{"id"},
		Partitions: 2, ParseWorkers: 1, Workers: 1,
		MemoryText: "64MiB", PartitionBufferText: "4KiB", MergeFanIn: 2, MaxRecordText: "1MiB",
	}
}
