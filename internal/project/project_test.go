package project

import (
	"path/filepath"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/dircompare"
	"github.com/ayame-editor/ayame-diff/internal/engine"
)

func TestProjectRoundTripResolvesRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "daily.ayamediff.json")
	p := Project{Mode: "csv", CSV: engine.Config{
		LeftPath: filepath.Join(dir, "data", "old.csv"), RightPath: filepath.Join(dir, "data", "new.csv"),
		OutputPath: filepath.Join(dir, "reports", "diff.tsv"), KeyNames: []string{"id"},
		IgnoreColumnNames: []string{"updated"}, Tolerance: 0.1, ToleranceSet: true,
	}}
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version || got.CSV.LeftPath != p.CSV.LeftPath || got.CSV.OutputPath != p.CSV.OutputPath || !got.CSV.ToleranceSet {
		t.Fatalf("project=%+v", got)
	}
}

func TestDirectoryProjectRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "config", "folder.ayamediff.json")
	want := &dircompare.DirectoryProject{
		Old: filepath.Join(root, "old"), New: filepath.Join(root, "new"),
		Filter: `size > 1MiB`, FilterSets: []string{"development"}, CompareBy: "hash", Workers: 4,
	}
	if err := Save(path, Project{Mode: "dir", Directory: want}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "dir" || got.Directory == nil || got.Directory.Old != want.Old || got.Directory.New != want.New || got.Directory.Filter != want.Filter || got.Directory.CompareBy != "hash" {
		t.Fatalf("project=%+v", got)
	}
}

func TestProjectRoundTripsFilterBuilderTrees(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "filtered.ayamediff.json")
	want := engine.Config{
		LeftPath: filepath.Join(dir, "old.csv"), RightPath: filepath.Join(dir, "new.csv"),
		OutputPath: filepath.Join(dir, "diff.tsv"),
		RowFilter: &engine.RowFilter{
			Match:      "all",
			Conditions: []engine.RowCondition{{Column: "status", Op: "eq", Value: "active"}},
			Groups:     []engine.RowFilter{{Match: "any", Conditions: []engine.RowCondition{{Column: "amount", Op: "gt", Value: "1000"}}}},
		},
		ColumnFilter: &engine.RowFilter{
			Conditions: []engine.RowCondition{{Op: "starts", Value: "tmp_"}},
		},
	}
	if err := Save(path, Project{Mode: "csv", CSV: want}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.CSV.RowFilter == nil || got.CSV.RowFilter.Conditions[0].Column != "status" ||
		len(got.CSV.RowFilter.Groups) != 1 || got.CSV.RowFilter.Groups[0].Match != "any" {
		t.Fatalf("row filter did not round trip: %#v", got.CSV.RowFilter)
	}
	if got.CSV.ColumnFilter == nil || got.CSV.ColumnFilter.Conditions[0].Value != "tmp_" {
		t.Fatalf("column filter did not round trip: %#v", got.CSV.ColumnFilter)
	}
}

func TestProjectRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := Save(path, Project{Version: 99}); err == nil {
		t.Fatal("unsupported version saved")
	}
}
