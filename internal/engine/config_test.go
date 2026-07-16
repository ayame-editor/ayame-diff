package engine

import (
	"reflect"
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

func TestConfigResolveAppliesIndexBaseWithoutMutation(t *testing.T) {
	t.Parallel()
	cfg := validConfigForValidation()
	cfg.IndexBase = 1
	cfg.KeyIndexes = []int{2}
	wantKeys := append([]int(nil), cfg.KeyIndexes...)

	resolved, err := cfg.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resolved.KeyIndexes, []int{1}) {
		t.Fatalf("resolved indexes = %#v", resolved.KeyIndexes)
	}
	if !reflect.DeepEqual(cfg.KeyIndexes, wantKeys) {
		t.Fatalf("resolve mutated caller: keys %#v", cfg.KeyIndexes)
	}
	resolved.KeyIndexes[0] = 99
	if cfg.KeyIndexes[0] != wantKeys[0] {
		t.Fatal("resolved indexes alias caller-owned slice")
	}

	excluded := validConfigForValidation()
	excluded.IndexBase = 1
	excluded.ExcludeKeyIndexes = []int{1, 3}
	resolvedExcluded, err := excluded.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resolvedExcluded.ExcludeKeyIndexes, []int{0, 2}) ||
		!reflect.DeepEqual(excluded.ExcludeKeyIndexes, []int{1, 3}) {
		t.Fatalf("excluded indexes: resolved=%#v caller=%#v", resolvedExcluded.ExcludeKeyIndexes, excluded.ExcludeKeyIndexes)
	}
}

func TestConfigValidateIsIdempotent(t *testing.T) {
	t.Parallel()
	cfg := validConfigForValidation()
	cfg.IndexBase = 1
	cfg.KeyIndexes = []int{3}
	want := append([]int(nil), cfg.KeyIndexes...)
	for i := 0; i < 2; i++ {
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate call %d: %v", i+1, err)
		}
	}
	if !reflect.DeepEqual(cfg.KeyIndexes, want) {
		t.Fatalf("Validate mutated indexes to %#v, want %#v", cfg.KeyIndexes, want)
	}
	first, err := cfg.resolve()
	if err != nil {
		t.Fatal(err)
	}
	second, err := cfg.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.KeyIndexes, second.KeyIndexes) || !reflect.DeepEqual(first.KeyIndexes, []int{2}) {
		t.Fatalf("resolve is not idempotent: first=%#v second=%#v", first.KeyIndexes, second.KeyIndexes)
	}
}

func TestSharedResourceValidators(t *testing.T) {
	t.Parallel()
	for _, n := range []int{MinPartitions, 8, MaxPartitions} {
		if err := ValidatePartitions(n); err != nil {
			t.Fatalf("ValidatePartitions(%d): %v", n, err)
		}
	}
	for _, n := range []int{0, 3, MaxPartitions + 1} {
		if err := ValidatePartitions(n); err == nil {
			t.Fatalf("ValidatePartitions(%d) succeeded", n)
		}
	}
	if err := ValidateMergeFanIn(MinMergeFanIn); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMergeFanIn(MaxMergeFanIn + 1); err == nil {
		t.Fatal("out-of-range merge fan-in succeeded")
	}
}

// TestConfigRejectsExcessiveWorkers covers #171: worker counts have an upper
// bound so a crafted request can't spawn workers without limit. MaxWorkers must
// still be accepted (given enough memory for the per-worker minimum).
func TestConfigRejectsExcessiveWorkers(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name        string
		parse, work int
		wantErr     bool
	}{
		{"at max", MaxWorkers, MaxWorkers, false},
		{"parse over max", MaxWorkers + 1, 1, true},
		{"workers over max", 1, MaxWorkers + 1, true},
	} {
		cfg := validConfigForValidation()
		cfg.ParseWorkers, cfg.Workers = c.parse, c.work
		cfg.MemoryText = "64GiB" // enough for MaxWorkers * MinMemoryBytesPerWorker
		err := cfg.Validate()
		if c.wantErr && (err == nil || !strings.Contains(err.Error(), "at most")) {
			t.Errorf("%s: Validate() = %v, want an 'at most' error", c.name, err)
		}
		if !c.wantErr && err != nil {
			t.Errorf("%s: Validate() = %v, want nil", c.name, err)
		}
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
