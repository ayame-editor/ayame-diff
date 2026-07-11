package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInspectInputsReadsHeadersAndDetectsReorder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.tsv")
	if err := os.WriteFile(left, []byte("id,name,value\n1,a,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("value\tid\tname\n2\t1\ta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := InspectInputs(Config{
		LeftPath: left, RightPath: right, HasHeader: true, AlignColumnsByName: true,
		LeftFormat: "auto", RightFormat: "auto", LeftParser: "auto", RightParser: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Header, []string{"id", "name", "value"}) || got.ColumnCount != 3 ||
		!got.ColumnsReordered || got.LeftFormat != "CSV" || got.RightFormat != "TSV" {
		t.Fatalf("inspection=%+v", got)
	}
}

func TestInspectInputsHeaderlessUsesSyntheticColumns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.tsv"), filepath.Join(dir, "right.tsv")
	if err := os.WriteFile(left, []byte("1\ta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("1\tb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := InspectInputs(Config{LeftPath: left, RightPath: right, HasHeader: false, LeftFormat: "auto", RightFormat: "auto", LeftParser: "auto", RightParser: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Header, []string{"column_0", "column_1"}) {
		t.Fatalf("inspection=%+v", got)
	}
}
