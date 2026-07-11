package main

import (
	"reflect"
	"testing"
)

func TestParseFlagsDefaultsToAllKeys(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{"--left", "a.tsv", "--right", "b.tsv", "--out", "d.tsv"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.ShowVersion {
		t.Fatal("showVersion = true")
	}
	cfg := opts.Engine
	if len(cfg.KeyNames) != 0 || len(cfg.KeyIndexes) != 0 || len(cfg.ExcludeKeyNames) != 0 || len(cfg.ExcludeKeyIndexes) != 0 {
		t.Fatalf("unexpected key options: %#v", cfg)
	}
}

func TestParseFlagsExclusionOptions(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{
		"--left", "a.csv", "--right", "b.csv", "--out", "d.tsv",
		"--exclude-key", "updated_at", "--exclude-key", "checksum",
		"--exclude-key-index", "4", "--index-base", "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := opts.Engine
	if !reflect.DeepEqual(cfg.ExcludeKeyNames, []string{"updated_at", "checksum"}) {
		t.Fatalf("exclude names = %#v", cfg.ExcludeKeyNames)
	}
	if !reflect.DeepEqual(cfg.ExcludeKeyIndexes, []int{4}) {
		t.Fatalf("exclude indexes = %#v", cfg.ExcludeKeyIndexes)
	}
}

func TestParseFlagsKeepsCLIOptionsOutsideEngineConfig(t *testing.T) {
	t.Parallel()
	opts, err := parseFlags([]string{
		"--left", "a.tsv", "--right", "b.tsv", "--out", "d.tsv",
		"--summary-json", "summary.json", "--diff-exit-code",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.SummaryJSON != "summary.json" || !opts.DiffExitCode {
		t.Fatalf("CLI options = %#v", opts)
	}
}
