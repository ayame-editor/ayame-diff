package engine

import (
	"strings"
	"testing"
)

func TestConfigRejectsIncludeAndExcludeKeyOptions(t *testing.T) {
	t.Parallel()
	cfg := validConfigForValidation()
	cfg.KeyNames = []string{"id"}
	cfg.ExcludeKeyNames = []string{"updated_at"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestResolvedAppliesIndexBaseToExcludedIndexes(t *testing.T) {
	t.Parallel()
	cfg := validConfigForValidation()
	cfg.IndexBase = 1
	cfg.ExcludeKeyIndexes = []int{1, 3}
	resolved, err := cfg.resolved()
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ExcludeKeyIndexes[0] != 0 || resolved.ExcludeKeyIndexes[1] != 2 {
		t.Fatalf("resolved excluded indexes = %#v", resolved.ExcludeKeyIndexes)
	}
	// Validate and resolved must leave the source config untouched.
	if cfg.ExcludeKeyIndexes[0] != 1 || cfg.ExcludeKeyIndexes[1] != 3 {
		t.Fatalf("source excluded indexes were mutated: %#v", cfg.ExcludeKeyIndexes)
	}
	if resolved.MemoryBytes == 0 || resolved.PartitionBufferBytes == 0 || resolved.MaxRecordBytes == 0 {
		t.Fatalf("resolved must populate derived byte sizes: %+v", resolved)
	}
}

func validConfigForValidation() Config {
	return Config{
		LeftPath: "left.tsv", RightPath: "right.tsv", OutputPath: "diff.tsv",
		HasHeader: true, AlignColumnsByName: true,
		LeftFormat: "auto", RightFormat: "auto",
		LeftParser: "auto", RightParser: "auto",
		Partitions: 2, ParseWorkers: 1, Workers: 1,
		MemoryText: "64MiB", PartitionBufferText: "4KiB",
		MergeFanIn: 2, MaxRecordText: "1MiB",
	}
}
