package engine

import (
	"context"
	"path/filepath"
	"testing"
)

func rowFilter(conditions ...RowCondition) *RowFilter {
	return &RowFilter{Conditions: conditions}
}

func cond(column, op, value string) RowCondition {
	return RowCondition{Column: column, Op: op, Value: value}
}

func TestRowFilterOperators(t *testing.T) {
	t.Parallel()
	header := []string{"status", "amount", "name"}
	mapping := []int{0, 1, 2}

	cases := []struct {
		name   string
		filter *RowFilter
		row    []string
		want   bool
	}{
		{"eq keeps a match", rowFilter(cond("status", filterOpEq, "active")), []string{"active", "10", "x"}, true},
		{"eq drops a mismatch", rowFilter(cond("status", filterOpEq, "active")), []string{"off", "10", "x"}, false},
		{"ne", rowFilter(cond("status", filterOpNe, "active")), []string{"off", "10", "x"}, true},
		{"contains", rowFilter(cond("name", filterOpContains, "EST")), []string{"a", "1", "TEST"}, true},
		{"starts", rowFilter(cond("name", filterOpStarts, "TE")), []string{"a", "1", "TEST"}, true},
		{"ends", rowFilter(cond("name", filterOpEnds, "ST")), []string{"a", "1", "TEST"}, true},
		{"gt numeric", rowFilter(cond("amount", filterOpGt, "1000")), []string{"a", "1001", "x"}, true},
		{"gt numeric miss", rowFilter(cond("amount", filterOpGt, "1000")), []string{"a", "999", "x"}, false},
		{"ge numeric", rowFilter(cond("amount", filterOpGe, "1000")), []string{"a", "1000", "x"}, true},
		{"lt numeric", rowFilter(cond("amount", filterOpLt, "1000")), []string{"a", "999", "x"}, true},
		{"le numeric", rowFilter(cond("amount", filterOpLe, "1000")), []string{"a", "1000", "x"}, true},
		{"between inclusive", rowFilter(RowCondition{Column: "amount", Op: filterOpBetween, Value: "5", Value2: "10"}), []string{"a", "10", "x"}, true},
		{"between miss", rowFilter(RowCondition{Column: "amount", Op: filterOpBetween, Value: "5", Value2: "10"}), []string{"a", "11", "x"}, false},
		{"between unordered bounds", rowFilter(RowCondition{Column: "amount", Op: filterOpBetween, Value: "10", Value2: "5"}), []string{"a", "6", "x"}, true},
		{"empty", rowFilter(cond("name", filterOpEmpty, "")), []string{"a", "1", ""}, true},
		{"not_empty", rowFilter(cond("name", filterOpNotEmpty, "")), []string{"a", "1", "z"}, true},
		{"regex", rowFilter(cond("name", filterOpRegex, "^TE")), []string{"a", "1", "TEST"}, true},
		{"regex miss", rowFilter(cond("name", filterOpRegex, "^TE")), []string{"a", "1", "test"}, false},
		{"not flag", rowFilter(RowCondition{Column: "status", Op: filterOpEq, Value: "active", Not: true}), []string{"active", "1", "x"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compiled, err := compileRowFilter(header, tc.filter, false, "row filter")
			if err != nil {
				t.Fatal(err)
			}
			if got := matchFilterFields(compiled, tc.row, mapping); got != tc.want {
				t.Fatalf("match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRowFilterIgnoreCaseAndNesting(t *testing.T) {
	t.Parallel()
	header := []string{"status", "region", "name"}
	mapping := []int{0, 1, 2}
	filter := &RowFilter{
		Match:      "all",
		IgnoreCase: true,
		Conditions: []RowCondition{cond("status", filterOpEq, "ACTIVE")},
		Groups: []RowFilter{{
			Match:      "any",
			Conditions: []RowCondition{cond("region", filterOpEq, "east"), cond("region", filterOpEq, "west")},
		}},
	}
	compiled, err := compileRowFilter(header, filter, false, "row filter")
	if err != nil {
		t.Fatal(err)
	}
	if !matchFilterFields(compiled, []string{"active", "east", "x"}, mapping) {
		t.Fatal("case-insensitive group AND should match")
	}
	if matchFilterFields(compiled, []string{"active", "north", "x"}, mapping) {
		t.Fatal("region outside the OR group should not match")
	}
	if matchFilterFields(compiled, []string{"off", "east", "x"}, mapping) {
		t.Fatal("status mismatch should not match")
	}
	// A whole-group NOT inverts the OR result.
	filter.Groups[0].Not = true
	compiled, err = compileRowFilter(header, filter, false, "row filter")
	if err != nil {
		t.Fatal(err)
	}
	if matchFilterFields(compiled, []string{"active", "east", "x"}, mapping) {
		t.Fatal("negated group should not match east")
	}
}

func TestCompileRowFilterRejectsInvalidTrees(t *testing.T) {
	t.Parallel()
	header := []string{"status", "amount"}
	cases := []struct {
		name   string
		filter *RowFilter
	}{
		{"unknown operator", rowFilter(cond("status", "wat", "x"))},
		{"unknown column", rowFilter(cond("missing", filterOpEq, "x"))},
		{"missing column", rowFilter(RowCondition{Op: filterOpEq, Value: "x"})},
		{"bad regex", rowFilter(cond("status", filterOpRegex, "("))},
		{"unknown match", &RowFilter{Match: "both", Conditions: []RowCondition{cond("status", filterOpEq, "x")}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := compileRowFilter(header, tc.filter, false, "row filter"); err == nil {
				t.Fatal("expected an error")
			}
		})
	}

	deep := &RowFilter{}
	node := deep
	for i := 0; i < filterMaxDepth+2; i++ {
		node.Groups = []RowFilter{{Conditions: []RowCondition{cond("status", filterOpEq, "x")}}}
		node = &node.Groups[0]
	}
	if _, err := compileRowFilter(header, deep, false, "row filter"); err == nil {
		t.Fatal("expected a depth error")
	}
}

func TestColumnFilterIgnoresMatchingColumns(t *testing.T) {
	t.Parallel()
	header := []string{"id", "tmp_value", "tmp_stamp", "amount"}
	cfg := Config{
		ColumnFilter:     &RowFilter{Conditions: []RowCondition{cond("", filterOpStarts, "tmp_")}},
		IgnoreWhitespace: "none",
		ColumnTolerances: nil,
	}
	comparison, err := buildComparisonConfig(header, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !comparison.ignoreColumns[1] || !comparison.ignoreColumns[2] {
		t.Fatalf("matching columns were not ignored: %#v", comparison.ignoreColumns)
	}
	if comparison.ignoreColumns[0] || comparison.ignoreColumns[3] {
		t.Fatalf("non-matching columns were ignored: %#v", comparison.ignoreColumns)
	}
	if !comparison.enabled {
		t.Fatal("a column filter must enable the comparison path")
	}
}

func TestPreviewFilterCountsRowsAndColumns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.csv")
	right := filepath.Join(dir, "right.csv")
	mustWriteFile(t, left, "id,region,tmp_x,value\n1,east,a,10\n2,west,b,20\n3,east,c,30\n")
	mustWriteFile(t, right, "id,region,tmp_x,value\n1,east,a,11\n2,west,b,20\n")

	cfg := testConfig(left, right, filepath.Join(dir, "out.tsv"))
	cfg.RowFilter = rowFilter(cond("region", filterOpEq, "east"))
	cfg.ColumnFilter = &RowFilter{Conditions: []RowCondition{cond("", filterOpStarts, "tmp_")}}
	preview, err := PreviewFilter(context.Background(), cfg, 100)
	if err != nil {
		t.Fatal(err)
	}
	if preview.LeftTotal != 3 || preview.LeftMatched != 2 {
		t.Fatalf("left counts = %d/%d, want 2/3", preview.LeftMatched, preview.LeftTotal)
	}
	if preview.RightTotal != 2 || preview.RightMatched != 1 {
		t.Fatalf("right counts = %d/%d, want 1/2", preview.RightMatched, preview.RightTotal)
	}
	if preview.KeptColumns != 3 {
		t.Fatalf("kept columns = %d, want 3", preview.KeptColumns)
	}
	if len(preview.IgnoredColumns) != 1 || preview.IgnoredColumns[0] != "tmp_x" {
		t.Fatalf("ignored columns = %#v", preview.IgnoredColumns)
	}
	if preview.Truncated {
		t.Fatal("a small input should not be truncated")
	}
}

func TestPreviewFilterTruncatesAtLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.csv")
	right := filepath.Join(dir, "right.csv")
	mustWriteFile(t, left, "id\n1\n2\n3\n")
	mustWriteFile(t, right, "id\n1\n")
	cfg := testConfig(left, right, filepath.Join(dir, "out.tsv"))
	preview, err := PreviewFilter(context.Background(), cfg, 2)
	if err != nil {
		t.Fatal(err)
	}
	if preview.LeftTotal != 2 || !preview.Truncated {
		t.Fatalf("preview = %+v, want a truncated left scan of 2", preview)
	}
}

func TestRunCSVRowFilterDropsRowsBeforeComparison(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.csv")
	right := filepath.Join(dir, "right.csv")
	mustWriteFile(t, left, "id,region,value\n1,east,10\n2,west,20\n3,east,30\n")
	mustWriteFile(t, right, "id,region,value\n1,east,11\n2,west,20\n3,east,30\n")
	cfg := testConfig(left, right, filepath.Join(dir, "out.tsv"))
	cfg.KeyNames = []string{"id"}
	cfg.RowFilter = rowFilter(cond("region", filterOpEq, "east"))
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.LeftRows != 2 || summary.RightRows != 2 {
		t.Fatalf("filtered rows = %d/%d, want 2/2", summary.LeftRows, summary.RightRows)
	}
	if summary.EqualRows != 1 || summary.DiffRows != 2 {
		t.Fatalf("summary = %+v", summary)
	}
}
