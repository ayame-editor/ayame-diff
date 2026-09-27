package engine

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

// TestRunManualColumnMapPairsRenamedColumns pins the core of #119: a user can
// state "left column A is right column B" so a comparison succeeds even when no
// header name matches and name alignment cannot help.
func TestRunManualColumnMapPairsRenamedColumns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.csv")
	rightPath := filepath.Join(dir, "right.csv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,label,amount\n1,alpha,10\n")
	mustWriteFile(t, rightPath, "amt,key,tag\n10,1,alpha\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	// Right columns are deliberately in a different order and renamed.
	cfg.ColumnMap = []ColumnPair{{Left: 0, Right: 1}, {Left: 1, Right: 2}, {Left: 2, Right: 0}}
	cfg.KeyNames = []string{"id"}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.DiffRows != 0 {
		t.Fatalf("summary=%+v, want one equal row and no differences", summary)
	}
	records := readDelimitedFile(t, outPath, '\t')
	if !reflect.DeepEqual(records[0], []string{"_diff", "_side", "id", "label", "amount"}) {
		t.Fatalf("canonical header=%#v", records[0])
	}
	if len(records) != 1 {
		t.Fatalf("output=%#v, want only the header", records)
	}
}

// TestRunManualColumnMapSupportsMissingAndIgnoredColumns covers the other two
// entry kinds: a column present on only one side, and a column kept for display
// but excluded from comparison.
func TestRunManualColumnMapSupportsMissingAndIgnoredColumns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.csv")
	rightPath := filepath.Join(dir, "right.csv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,value,note\n1,10,hello\n")
	mustWriteFile(t, rightPath, "id,val\n1,10\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.ColumnMap = []ColumnPair{{Left: 0, Right: 0}, {Left: 1, Right: 1}, {Left: 2, Right: -1}}
	cfg.KeyNames = []string{"id"}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	// The right side has no "note" column, so it reads empty and differs.
	if summary.DiffRows != 2 || summary.ChangedLeft != 1 || summary.ChangedRight != 1 {
		t.Fatalf("missing-tolerant summary=%+v", summary)
	}

	// Marking the one-sided column ignored makes the rows equal and keeps the
	// column out of the comparison while it stays in the output row.
	cfg.ColumnMap[2].Ignore = true
	summary, err = Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.DiffRows != 0 {
		t.Fatalf("ignored one-sided summary=%+v", summary)
	}
	records := readDelimitedFile(t, outPath, '\t')
	if !reflect.DeepEqual(records[0], []string{"_diff", "_side", "id", "value", "note"}) {
		t.Fatalf("canonical header=%#v", records[0])
	}
}

// TestInspectInputsReportsMappedHeader checks that a client can inspect with a
// map and get the canonical header and column count back.
func TestInspectInputsReportsMappedHeader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.csv")
	rightPath := filepath.Join(dir, "right.csv")
	mustWriteFile(t, leftPath, "id,name\n1,alpha\n")
	mustWriteFile(t, rightPath, "name,id\nalpha,1\n")

	got, err := InspectInputs(Config{
		LeftPath: leftPath, RightPath: rightPath, HasHeader: true,
		LeftFormat: "auto", RightFormat: "auto", LeftParser: "auto", RightParser: "auto",
		ColumnMap: []ColumnPair{{Left: 0, Right: 1}, {Left: 1, Right: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Header, []string{"id", "name"}) || got.ColumnCount != 2 {
		t.Fatalf("inspection=%+v", got)
	}
}

// TestColumnMapValidation rejects maps that could not describe a real pairing
// before any input is read.
func TestColumnMapValidation(t *testing.T) {
	t.Parallel()
	base := testConfig("left.csv", "right.csv", "out.tsv")
	cases := []struct {
		name string
		m    []ColumnPair
	}{
		{"both absent", []ColumnPair{{Left: -1, Right: -1}}},
		{"duplicate left", []ColumnPair{{Left: 0, Right: 0}, {Left: 0, Right: 1}}},
		{"duplicate right", []ColumnPair{{Left: 0, Right: 0}, {Left: 1, Right: 0}}},
		{"index below -1", []ColumnPair{{Left: -2, Right: 0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			cfg.ColumnMap = tc.m
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate accepted %#v", tc.m)
			}
		})
	}
}

// TestRunRejectsOutOfRangeColumnMap checks the per-side bounds that need the
// inspected headers.
func TestRunRejectsOutOfRangeColumnMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.csv")
	rightPath := filepath.Join(dir, "right.csv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,name\n1,alpha\n")
	mustWriteFile(t, rightPath, "id,name\n1,alpha\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.ColumnMap = []ColumnPair{{Left: 0, Right: 0}, {Left: 1, Right: 5}}
	if _, err := Run(context.Background(), cfg); err == nil {
		t.Fatal("Run accepted a right index outside the right header")
	}
}
