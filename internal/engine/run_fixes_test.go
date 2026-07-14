package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Regression for issue #39: Validate and Run must not mutate the caller's
// Config. Previously Validate subtracted IndexBase in place, so validating
// or running twice silently shifted the key to a different column (or
// failed with "key index becomes negative").
func TestRunDoesNotMutateConfig(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.tsv")
	right := filepath.Join(dir, "right.tsv")
	mustWriteFile(t, left, "id\tvalue\n1\ta\n2\tb\n")
	mustWriteFile(t, right, "id\tvalue\n1\ta\n2\tc\n")

	cfg := testConfig(left, right, filepath.Join(dir, "out1.tsv"))
	cfg.IndexBase = 1
	cfg.KeyIndexes = []int{1} // 1-based: the "id" column

	if err := cfg.Validate(); err != nil {
		t.Fatalf("first Validate: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("second Validate must not double-apply --index-base: %v", err)
	}
	if cfg.KeyIndexes[0] != 1 {
		t.Fatalf("Validate mutated KeyIndexes: got %v, want [1]", cfg.KeyIndexes)
	}

	first, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if cfg.KeyIndexes[0] != 1 || cfg.IndexBase != 1 {
		t.Fatalf("Run mutated caller config: KeyIndexes=%v IndexBase=%d", cfg.KeyIndexes, cfg.IndexBase)
	}
	if first.Elapsed == "" {
		t.Fatal("Run must populate Summary.Elapsed")
	}

	cfg.OutputPath = filepath.Join(dir, "out2.tsv")
	second, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run with the same config: %v", err)
	}
	if first.DiffRows != second.DiffRows || first.ChangedLeft != second.ChangedLeft {
		t.Fatalf("second Run diverged: first=%+v second=%+v", first, second)
	}
	if second.DiffRows != 2 {
		t.Fatalf("DiffRows = %d, want 2 (one changed key emits left+right rows)", second.DiffRows)
	}
}

// Regression for issue #40: a real worker failure must not be masked by a
// sibling worker's context.Canceled.
func TestPreferRootCause(t *testing.T) {
	real := errors.New("disk full")
	canceled := fmt.Errorf("wrapped: %w", context.Canceled)
	cases := []struct {
		name               string
		current, candidate error
		want               error
	}{
		{"first error wins when real", real, canceled, real},
		{"context error is replaced by real cause", canceled, real, real},
		{"nil current takes candidate", nil, canceled, canceled},
		{"nil candidate keeps current", real, nil, real},
		{"two real errors keep the first", real, errors.New("later"), real},
		{"two context errors keep the first", canceled, context.Canceled, canceled},
	}
	for _, tc := range cases {
		if got := preferRootCause(tc.current, tc.candidate); got != tc.want {
			t.Errorf("%s: preferRootCause(%v, %v) = %v, want %v", tc.name, tc.current, tc.candidate, got, tc.want)
		}
	}
}

// Regression for issue #41: a header-only file without a trailing newline is
// a valid zero-row input on every parser path, not an EOF error.
func TestHeaderOnlyFileWithoutTrailingNewline(t *testing.T) {
	for _, tc := range []struct {
		name, ext, header string
	}{
		{"simple-tsv", ".tsv", "id\tvalue"},
		{"rfc-csv", ".csv", "id,value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			left := filepath.Join(dir, "left"+tc.ext)
			right := filepath.Join(dir, "right"+tc.ext)
			mustWriteFile(t, left, tc.header) // no trailing newline
			mustWriteFile(t, right, tc.header+"\n1,a\n")
			if tc.ext == ".tsv" {
				mustWriteFile(t, right, tc.header+"\n1\ta\n")
			}
			cfg := testConfig(left, right, filepath.Join(dir, "out.tsv"))
			summary, err := Run(context.Background(), cfg)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if summary.LeftRows != 0 || summary.RightRows != 1 || summary.RightOnly != 1 {
				t.Fatalf("summary = %+v, want left=0 right=1 right_only=1", summary)
			}
		})
	}
}

// Regression for issue #41: a UTF-8 BOM on a headerless input must not be
// encoded into the first row's key; BOM-vs-no-BOM files with identical
// content must compare equal on both parser paths.
func TestHeadlessBOMDoesNotProduceSpuriousDiff(t *testing.T) {
	for _, tc := range []struct {
		name, ext, body string
	}{
		{"simple-tsv", ".tsv", "1\ta\n2\tb\n"},
		{"rfc-csv", ".csv", "1,a\n2,b\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			left := filepath.Join(dir, "left"+tc.ext)
			right := filepath.Join(dir, "right"+tc.ext)
			mustWriteFile(t, left, "\uFEFF"+tc.body)
			mustWriteFile(t, right, tc.body)
			cfg := testConfig(left, right, filepath.Join(dir, "out.tsv"))
			cfg.HasHeader = false
			cfg.AlignColumnsByName = false
			summary, err := Run(context.Background(), cfg)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if summary.DiffRows != 0 {
				t.Fatalf("DiffRows = %d, want 0 (BOM must not reach the key); summary = %+v", summary.DiffRows, summary)
			}
			if summary.LeftRows != 2 || summary.RightRows != 2 {
				t.Fatalf("row counts = %d/%d, want 2/2", summary.LeftRows, summary.RightRows)
			}
		})
	}
}

// Regression for issue #41: a user-supplied --work-dir must survive the run;
// only its contents are cleaned up.
func TestUserWorkDirIsPreserved(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.tsv")
	right := filepath.Join(dir, "right.tsv")
	mustWriteFile(t, left, "id\tvalue\n1\ta\n")
	mustWriteFile(t, right, "id\tvalue\n1\ta\n")
	workDir := filepath.Join(dir, "user-work-dir")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(left, right, filepath.Join(dir, "out.tsv"))
	cfg.WorkDir = workDir
	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	info, err := os.Stat(workDir)
	if err != nil {
		t.Fatalf("user work dir was deleted: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("user work dir is not a directory")
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("work dir still contains %d entries after cleanup", len(entries))
	}
}

func TestFileDescriptorNeed(t *testing.T) {
	cfg := Config{Partitions: 256, Workers: 2, MergeFanIn: 32}
	if got := fileDescriptorNeed(cfg); got != 256+64 {
		t.Fatalf("need = %d, want %d (partitions dominate)", got, 256+64)
	}
	cfg = Config{Partitions: 4, Workers: 8, MergeFanIn: 256}
	// workers are capped by partitions: min(8,4)=4 workers * 256 fan-in.
	if got := fileDescriptorNeed(cfg); got != 4*256+64 {
		t.Fatalf("need = %d, want %d (merge fan-in dominates)", got, 4*256+64)
	}
}
