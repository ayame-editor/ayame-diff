package dircompare

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestResolveBuiltinAndExternalFilterSets(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "filters.json")
	data := []byte(`{
  "version": 1,
  "default": "logs",
  "filters": {"logs": {"includes": ["**/*.log"], "expression": "size > 1KB"}},
  "mode": "dir",
  "directory": {"excludes": ["private/**"], "filter_sets": ["vcs"], "compare_by": "hash"}
}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	set, project, err := ResolveFilterSets(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if project == nil || project.CompareBy != "hash" || !slices.Contains(set.Includes, "**/*.log") || !slices.Contains(set.Excludes, ".git/**") || !slices.Contains(set.Excludes, "private/**") || set.Expression != "size > 1KB" {
		t.Fatalf("set=%+v project=%+v", set, project)
	}

	set, _, err = ResolveFilterSets("", []string{"node", "rust"})
	if err != nil || !slices.Contains(set.Excludes, "node_modules/**") || !slices.Contains(set.Excludes, "target/**") {
		t.Fatalf("set=%+v err=%v", set, err)
	}
	if _, _, err := ResolveFilterSets("", []string{"missing"}); err == nil {
		t.Fatal("unknown set succeeded")
	}
}

func TestBuiltinFilterSetNamesSorted(t *testing.T) {
	t.Parallel()
	if got := BuiltinFilterSetNames(); !slices.IsSorted(got) || !slices.Contains(got, "development") {
		t.Fatalf("names=%v", got)
	}
}
