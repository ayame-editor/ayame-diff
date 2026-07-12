package engine

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRunMixedTSVCSVWithDuplicateKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.csv")
	outPath := filepath.Join(dir, "diff.tsv")

	left := "id\tregion\tname\tvalue\n" +
		"1\tJP\talpha\t10\n" +
		"2\tJP\tbeta\t20\n" +
		"2\tJP\tbeta\t20\n" +
		"3\tUS\tgamma\t30\n" +
		"4\tJP\told\t40\n"
	right := "region,id,name,value\n" +
		"JP,5,epsilon,50\n" +
		"JP,4,new,40\n" +
		"US,3,gamma,31\n" +
		"JP,2,beta,20\n" +
		"JP,2,beta,20\n"
	mustWriteFile(t, leftPath, left)
	mustWriteFile(t, rightPath, right)

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id", "region"}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	wantSummary := Summary{
		LeftRows: 5, RightRows: 5, EqualRows: 2,
		LeftOnly: 1, RightOnly: 1, ChangedLeft: 2, ChangedRight: 2,
		DiffRows: 6, Partitions: 8, Workers: 2,
	}
	summary.Elapsed = ""
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("summary = %#v, want %#v", summary, wantSummary)
	}

	records := readDelimitedFile(t, outPath, '\t')
	if len(records) != 7 {
		t.Fatalf("output records = %d, want 7: %#v", len(records), records)
	}
	if !reflect.DeepEqual(records[0], []string{"_diff", "_side", "id", "region", "name", "value"}) {
		t.Fatalf("header = %#v", records[0])
	}
	rows := records[1:]
	sort.Slice(rows, func(i, j int) bool {
		for k := range rows[i] {
			if rows[i][k] != rows[j][k] {
				return rows[i][k] < rows[j][k]
			}
		}
		return false
	})
	wantRows := [][]string{
		{"CHANGED", "left", "3", "US", "gamma", "30"},
		{"CHANGED", "left", "4", "JP", "old", "40"},
		{"CHANGED", "right", "3", "US", "gamma", "31"},
		{"CHANGED", "right", "4", "JP", "new", "40"},
		{"LEFT_ONLY", "left", "1", "JP", "alpha", "10"},
		{"RIGHT_ONLY", "right", "5", "JP", "epsilon", "50"},
	}
	sort.Slice(wantRows, func(i, j int) bool {
		for k := range wantRows[i] {
			if wantRows[i][k] != wantRows[j][k] {
				return wantRows[i][k] < wantRows[j][k]
			}
		}
		return false
	})
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("rows = %#v, want %#v", rows, wantRows)
	}
}

func TestRunCSVComparisonIgnoreOptionsPreserveOriginalRows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath, outPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,note,stamp\nA,hello   WORLD,time=100\n")
	mustWriteFile(t, rightPath, "id,note,stamp\na,hello world,time=200\n")
	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id"}
	cfg.IgnoreCase = true
	cfg.IgnoreWhitespace = "change"
	cfg.LineFilters = []string{`time=\d+`}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.DiffRows != 0 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestRunCSVIgnoreColumnAlsoLeavesDefaultKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath, outPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,updated,value\n1,10:00,ok\n")
	mustWriteFile(t, rightPath, "id,updated,value\n1,11:00,ok\n")
	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.IgnoreColumnNames = []string{"updated"}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.DiffRows != 0 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestRunCSVNumericToleranceMatchesDuplicateKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath, outPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,value\n1,10.0000\n1,20\n")
	mustWriteFile(t, rightPath, "id,value\n1,20.00005\n1,11\n")
	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id"}
	cfg.Tolerance, cfg.ToleranceSet = 0.001, true
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.ChangedLeft != 1 || summary.ChangedRight != 1 {
		t.Fatalf("summary=%+v", summary)
	}
	records := readDelimitedFile(t, outPath, '\t')
	if len(records) != 3 || records[1][3] != "10.0000" || records[2][3] != "11" {
		t.Fatalf("original output rows=%#v", records)
	}
}

func TestRunCSVColumnToleranceIsRemovedFromDefaultKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath, outPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,value\n1,10.00\n")
	mustWriteFile(t, rightPath, "id,value\n1,10.05\n")
	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.ColumnTolerances = []ColumnTolerance{{Name: "value", Value: 0.1}}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.DiffRows != 0 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestRunCSVToleranceUsesMaximumDuplicateMatching(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath, outPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "diff.tsv")
	// Sorted order considers the exact 0.0 pair first, but fixing that pair is
	// wrong: 0.4 can only match right 0.0 while left 0.0 can match right -0.4.
	mustWriteFile(t, leftPath, "id,value\n1,0.0\n1,0.4\n")
	mustWriteFile(t, rightPath, "id,value\n1,0.0\n1,-0.4\n")
	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id"}
	cfg.Tolerance, cfg.ToleranceSet = 0.4, true
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 2 || summary.DiffRows != 0 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestRunCSVCellDiffTSVUsesToleranceAndRanking(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath, outPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,name,price,updated\n1,old,10.00,10:00\n")
	mustWriteFile(t, rightPath, "id,name,price,updated\n1,new,10.05,11:00\n")
	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id"}
	cfg.CellDiff = true
	cfg.IgnoreColumnNames = []string{"updated"}
	cfg.ColumnTolerances = []ColumnTolerance{{Name: "price", Value: 0.1}}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(summary.ColumnChanges, []ColumnChange{{Index: 1, Name: "name", Count: 1}}) {
		t.Fatalf("column ranking=%+v", summary.ColumnChanges)
	}
	records := readDelimitedFile(t, outPath, '\t')
	want := [][]string{
		{"_diff", "_side", "_changed_cols", "id", "name", "price", "updated"},
		{"CHANGED", "left", "name", "1", "old", "10.00", "10:00"},
		{"CHANGED", "right", "name", "1", "new", "10.05", "11:00"},
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records=%#v", records)
	}
}

func TestRunCSVCellDiffJSONLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath, outPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "diff.jsonl")
	mustWriteFile(t, leftPath, "id,name,value\n1,old,10\n2,left,20\n")
	mustWriteFile(t, rightPath, "id,name,value\n1,new,11\n3,right,30\n")
	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id"}
	cfg.CellDiff, cfg.OutputFormat = true, "jsonl"
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.ColumnChanges) != 2 {
		t.Fatalf("ranking=%+v", summary.ColumnChanges)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("json lines=%d: %s", len(lines), data)
	}
	var changed jsonRecordDiff
	for _, line := range lines {
		var item jsonRecordDiff
		if err := json.Unmarshal(line, &item); err != nil {
			t.Fatal(err)
		}
		if item.ID == "" {
			t.Fatalf("diff record has no stable merge ID: %s", line)
		}
		if item.Kind == diffChanged && len(item.Old) > 0 && len(item.New) > 0 {
			changed = item
		}
	}
	if len(changed.ChangedColumns) != 2 || changed.ChangedColumns[0].Name != "name" ||
		changed.ChangedColumns[0].Old != "old" || changed.ChangedColumns[0].New != "new" {
		t.Fatalf("changed=%+v", changed)
	}
}

func TestRunCSVReconcileUsesStableChoicesAndPreservesInputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath, rightPath := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	leftText := "id,name\n1,same\n2,left\n3,left-only\n"
	rightText := "id,name\n1,same\n2,right\n4,right-only\n"
	mustWriteFile(t, leftPath, leftText)
	mustWriteFile(t, rightPath, rightText)
	diffPath := filepath.Join(dir, "diff.jsonl")
	cfg := testConfig(leftPath, rightPath, diffPath)
	cfg.KeyNames, cfg.CellDiff, cfg.OutputFormat = []string{"id"}, true, "jsonl"
	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(diffPath)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(file)
	choices := make(map[string]string)
	for {
		var item jsonRecordDiff
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		choices[item.ID] = "right"
	}
	_ = file.Close()
	mergedPath := filepath.Join(dir, "merged.csv")
	cfg = testConfig(leftPath, rightPath, mergedPath)
	cfg.KeyNames, cfg.Reconcile, cfg.MergeChoices, cfg.OutputHeader = []string{"id"}, true, choices, true
	cfg.OutputDelimiter = ','
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.UnresolvedRows != 0 {
		t.Fatalf("summary=%+v", summary)
	}
	records := readDelimitedFile(t, mergedPath, ',')
	sort.Slice(records[1:], func(i, j int) bool { return records[1:][i][0] < records[1:][j][0] })
	want := [][]string{{"id", "name"}, {"1", "same"}, {"2", "right"}, {"4", "right-only"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records=%#v want=%#v", records, want)
	}
	for path, wantText := range map[string]string{leftPath: leftText, rightPath: rightText} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != wantText {
			t.Fatalf("input changed %s: %q err=%v", path, got, readErr)
		}
	}
}

func TestRunCSVReconcileRejectsUnresolvedWithoutCreatingOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right, output := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv"), filepath.Join(dir, "merged.csv")
	mustWriteFile(t, left, "id,name\n1,left\n")
	mustWriteFile(t, right, "id,name\n1,right\n")
	cfg := testConfig(left, right, output)
	cfg.KeyNames, cfg.Reconcile, cfg.OutputHeader, cfg.OutputDelimiter = []string{"id"}, true, true, ','
	if _, err := Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output exists after rejection: %v", err)
	}
}

func TestRunRFC4180MultilineAndGzipOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.csv.gz")
	rightPath := filepath.Join(dir, "right.csv")
	outPath := filepath.Join(dir, "diff.tsv.gz")
	mustWriteGzipFile(t, leftPath, "id,note\n1,\"hello, world\"\n2,\"line1\nline2\"\n")
	mustWriteFile(t, rightPath, "id,note\n2,\"line1\nline2\"\n1,\"hello, world\"\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id"}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 2 || summary.DiffRows != 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	reader := csv.NewReader(bufio.NewReader(gz))
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(records, [][]string{{"_diff", "_side", "id", "note"}}) {
		t.Fatalf("gzip output = %#v", records)
	}
}

func TestRunChangedMultilineFieldProducesValidTSV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.csv")
	rightPath := filepath.Join(dir, "right.csv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,note\n1,\"a\tb\nleft\"\n")
	mustWriteFile(t, rightPath, "id,note\n1,\"a\tb\nright\"\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.KeyNames = []string{"id"}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ChangedLeft != 1 || summary.ChangedRight != 1 || summary.DiffRows != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	records := readDelimitedFile(t, outPath, '\t')
	want := [][]string{
		{"_diff", "_side", "id", "note"},
		{"CHANGED", "left", "1", "a\tb\nleft"},
		{"CHANGED", "right", "1", "a\tb\nright"},
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records = %#v, want %#v", records, want)
	}
}

func TestRunHeaderlessIndexes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "a\t1\nb\t2\n")
	mustWriteFile(t, rightPath, "b\t3\na\t1\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.HasHeader = false
	cfg.AlignColumnsByName = false
	cfg.KeyIndexes = []int{0}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.ChangedLeft != 1 || summary.ChangedRight != 1 || summary.DiffRows != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	records := readDelimitedFile(t, outPath, '\t')
	if !reflect.DeepEqual(records[0], []string{"_diff", "_side", "column_0", "column_1"}) {
		t.Fatalf("header = %#v", records[0])
	}
}

func TestRunIsRepeatableAndSetsElapsed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	mustWriteFile(t, leftPath, "id\tvalue\n1\tsame\n")
	mustWriteFile(t, rightPath, "id\tvalue\n1\tsame\n")

	cfg := testConfig(leftPath, rightPath, filepath.Join(dir, "first.tsv"))
	cfg.IndexBase = 1
	cfg.KeyIndexes = []int{1}
	for i, output := range []string{"first.tsv", "second.tsv"} {
		cfg.OutputPath = filepath.Join(dir, output)
		summary, err := Run(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Run %d: %v", i+1, err)
		}
		if summary.EqualRows != 1 || summary.DiffRows != 0 {
			t.Fatalf("Run %d summary: %#v", i+1, summary)
		}
		if summary.Elapsed == "" {
			t.Fatalf("Run %d did not set elapsed time", i+1)
		}
	}
	if !reflect.DeepEqual(cfg.KeyIndexes, []int{1}) {
		t.Fatalf("Run mutated caller indexes: %#v", cfg.KeyIndexes)
	}
}

func TestRunHeaderOnlyWithoutTrailingNewline(t *testing.T) {
	t.Parallel()
	for _, parser := range []string{"simple", "rfc4180"} {
		parser := parser
		t.Run(parser, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			leftPath := filepath.Join(dir, "left.csv")
			rightPath := filepath.Join(dir, "right.csv")
			outPath := filepath.Join(dir, "diff.tsv")
			mustWriteFile(t, leftPath, "id,value")
			mustWriteFile(t, rightPath, "id,value")

			cfg := testConfig(leftPath, rightPath, outPath)
			cfg.LeftParser = parser
			cfg.RightParser = parser
			cfg.ParseWorkers = 1
			summary, err := Run(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if summary.LeftRows != 0 || summary.RightRows != 0 || summary.DiffRows != 0 {
				t.Fatalf("unexpected summary: %#v", summary)
			}
		})
	}
}

func TestRunHeaderlessUTF8BOM(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parser     string
		compressed bool
	}{
		{name: "simple sequential", parser: "simple"},
		{name: "simple parallel", parser: "simple"},
		{name: "simple gzip", parser: "simple", compressed: true},
		{name: "rfc4180", parser: "rfc4180"},
		{name: "rfc4180 gzip", parser: "rfc4180", compressed: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			ext := ".csv"
			if tt.compressed {
				ext += ".gz"
			}
			leftPath := filepath.Join(dir, "left"+ext)
			rightPath := filepath.Join(dir, "right"+ext)
			outPath := filepath.Join(dir, "diff.tsv")
			left := utf8BOM + "1,alpha\n2,beta\n"
			right := "1,alpha\n2,beta\n"
			if tt.compressed {
				mustWriteGzipFile(t, leftPath, left)
				mustWriteGzipFile(t, rightPath, right)
			} else {
				mustWriteFile(t, leftPath, left)
				mustWriteFile(t, rightPath, right)
			}

			cfg := testConfig(leftPath, rightPath, outPath)
			cfg.HasHeader = false
			cfg.AlignColumnsByName = false
			cfg.LeftParser = tt.parser
			cfg.RightParser = tt.parser
			if tt.name == "simple sequential" {
				cfg.ParseWorkers = 1
			}
			summary, err := Run(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if summary.EqualRows != 2 || summary.DiffRows != 0 {
				t.Fatalf("BOM created a false difference: %#v", summary)
			}
		})
	}
}

func TestRunPreservesExplicitWorkDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "diff.tsv")
	workPath := filepath.Join(dir, "user-work")
	mustWriteFile(t, leftPath, "id\n1\n")
	mustWriteFile(t, rightPath, "id\n1\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.WorkDir = workPath
	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(workPath)
	if err != nil {
		t.Fatalf("explicit work directory was removed: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("explicit work path is not a directory: %s", workPath)
	}
	entries, err := os.ReadDir(workPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("explicit work directory still has generated contents: %v", entries)
	}
}

func TestRunDefaultsToAllColumnsAsKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id\tvalue\n1\talpha\n2\told\n")
	mustWriteFile(t, rightPath, "id\tvalue\n1\talpha\n2\tnew\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.LeftOnly != 1 || summary.RightOnly != 1 || summary.ChangedLeft != 0 || summary.ChangedRight != 0 || summary.DiffRows != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	records := readDelimitedFile(t, outPath, '\t')
	want := [][]string{
		{"_diff", "_side", "id", "value"},
		{"LEFT_ONLY", "left", "2", "old"},
		{"RIGHT_ONLY", "right", "2", "new"},
	}
	sort.Slice(records[1:], func(i, j int) bool {
		return records[1:][i][0] < records[1:][j][0]
	})
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records = %#v, want %#v", records, want)
	}
}

func TestRunExcludeKeyProducesChangedRows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.csv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "id,region,value,updated_at\n1,JP,100,old\n2,US,200,same\n")
	mustWriteFile(t, rightPath, "updated_at\tvalue\tregion\tid\nnew\t100\tJP\t1\nsame\t200\tUS\t2\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.ExcludeKeyNames = []string{"updated_at"}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EqualRows != 1 || summary.ChangedLeft != 1 || summary.ChangedRight != 1 || summary.LeftOnly != 0 || summary.RightOnly != 0 || summary.DiffRows != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	records := readDelimitedFile(t, outPath, '\t')
	want := [][]string{
		{"_diff", "_side", "id", "region", "value", "updated_at"},
		{"CHANGED", "left", "1", "JP", "100", "old"},
		{"CHANGED", "right", "1", "JP", "100", "new"},
	}
	sort.Slice(records[1:], func(i, j int) bool {
		rows := records[1:]
		for k := range rows[i] {
			if rows[i][k] != rows[j][k] {
				return rows[i][k] < rows[j][k]
			}
		}
		return false
	})
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records = %#v, want %#v", records, want)
	}
}

func TestRunHeaderlessExcludeKeyIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.tsv")
	rightPath := filepath.Join(dir, "right.tsv")
	outPath := filepath.Join(dir, "diff.tsv")
	mustWriteFile(t, leftPath, "a\told\n")
	mustWriteFile(t, rightPath, "a\tnew\n")

	cfg := testConfig(leftPath, rightPath, outPath)
	cfg.HasHeader = false
	cfg.AlignColumnsByName = false
	cfg.ExcludeKeyIndexes = []int{1}
	summary, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ChangedLeft != 1 || summary.ChangedRight != 1 || summary.DiffRows != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestExternalSortProducesOrderedRecords(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	partitionPath := filepath.Join(dir, "part.bin")
	f, err := os.Create(partitionPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(f)
	for i := 99; i >= 0; i-- {
		key, row, err := encodeStringFields([]string{string(rune('a' + i%5)), string(rune(i + 1000))}, []int{0, 1}, []int{0}, false, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeBinRecord(writer, binRecord{Key: append([]byte(nil), key...), Row: append([]byte(nil), row...)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	sortedPath, err := makeSortedFile(context.Background(), partitionPath, filepath.Join(dir, "sort"), "test", 512, 3, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	sortedFile, err := os.Open(sortedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sortedFile.Close()
	reader := newBinRecordReader(sortedFile, 4096, 1024*1024)
	var previous binRecord
	count := 0
	for {
		record, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			if string(previous.Key) > string(record.Key) || (string(previous.Key) == string(record.Key) && string(previous.Row) > string(record.Row)) {
				t.Fatalf("records are not sorted at %d", count)
			}
		}
		previous = record
		count++
	}
	if count != 100 {
		t.Fatalf("record count = %d, want 100", count)
	}
}

func testConfig(left, right, out string) Config {
	return Config{
		LeftPath: left, RightPath: right, OutputPath: out,
		HasHeader: true, AlignColumnsByName: true,
		LeftFormat: "auto", RightFormat: "auto",
		LeftParser: "auto", RightParser: "auto",
		Partitions: 8, ParseWorkers: 2, Workers: 2,
		MemoryText: "64MiB", PartitionBufferText: "16KiB",
		MergeFanIn: 4, MaxRecordText: "8MiB",
		Progress: false, OutputHeader: true,
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteGzipFile(t *testing.T, path, contents string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(contents)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func readDelimitedFile(t *testing.T, path string, delimiter rune) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(bufio.NewReader(f))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records
}
