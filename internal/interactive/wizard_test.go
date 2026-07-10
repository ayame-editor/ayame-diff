package interactive

import (
	"reflect"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/engine"
)

func TestNormalizePathInputQuotes(t *testing.T) {
	t.Parallel()
	got := normalizePathInput(`  "C:\data\left file.tsv"  `)
	if got != `C:\data\left file.tsv` {
		t.Fatalf("normalizePathInput = %q", got)
	}
}

func TestInitialColumnSelectionHonorsIndexBase(t *testing.T) {
	t.Parallel()
	cfg := engine.Config{KeyIndexes: []int{1, 3}, IndexBase: 1}
	got := initialColumnSelection(cfg, []string{"id", "name", "value"}, keyModeInclude)
	want := []bool{true, false, true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %#v, want %#v", got, want)
	}
}

func TestApplyColumnSelectionUsesHeaderNames(t *testing.T) {
	t.Parallel()
	cfg := engine.Config{HasHeader: true, IndexBase: 1, ExcludeKeyNames: []string{"old"}}
	applyColumnSelection(&cfg, []string{"id", "updated_at", "value"}, []bool{true, false, true}, keyModeInclude)
	if !reflect.DeepEqual(cfg.KeyNames, []string{"id", "value"}) {
		t.Fatalf("KeyNames = %#v", cfg.KeyNames)
	}
	if len(cfg.ExcludeKeyNames) != 0 || len(cfg.ExcludeKeyIndexes) != 0 {
		t.Fatalf("exclude options not cleared: %#v", cfg)
	}
	if cfg.IndexBase != 0 {
		t.Fatalf("IndexBase = %d, want 0", cfg.IndexBase)
	}
}

func TestApplyColumnSelectionFallsBackToIndexForEmptyHeader(t *testing.T) {
	t.Parallel()
	cfg := engine.Config{HasHeader: true}
	applyColumnSelection(&cfg, []string{"id", ""}, []bool{false, true}, keyModeExclude)
	if !reflect.DeepEqual(cfg.ExcludeKeyIndexes, []int{1}) {
		t.Fatalf("ExcludeKeyIndexes = %#v", cfg.ExcludeKeyIndexes)
	}
}

func TestValidateDelimiterText(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "comma", "tab", `\t`, "pipe", ";"} {
		if err := validateDelimiterText(value); err != nil {
			t.Fatalf("validateDelimiterText(%q): %v", value, err)
		}
	}
	for _, value := range []string{"日本", "\n", `"`} {
		if err := validateDelimiterText(value); err == nil {
			t.Fatalf("validateDelimiterText(%q) unexpectedly succeeded", value)
		}
	}
}

func TestCloneConfigCopiesSlices(t *testing.T) {
	t.Parallel()
	original := engine.Config{KeyNames: []string{"id"}, KeyIndexes: []int{2}}
	clone := cloneConfig(original)
	clone.KeyNames[0] = "changed"
	clone.KeyIndexes[0] = 9
	if original.KeyNames[0] != "id" || original.KeyIndexes[0] != 2 {
		t.Fatalf("clone modified original: %#v", original)
	}
}
