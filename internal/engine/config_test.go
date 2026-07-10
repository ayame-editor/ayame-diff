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

func TestConfigAppliesIndexBaseToExcludedIndexes(t *testing.T) {
	t.Parallel()
	cfg := validConfigForValidation()
	cfg.IndexBase = 1
	cfg.ExcludeKeyIndexes = []int{1, 3}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.ExcludeKeyIndexes[0] != 0 || cfg.ExcludeKeyIndexes[1] != 2 {
		t.Fatalf("excluded indexes = %#v", cfg.ExcludeKeyIndexes)
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
