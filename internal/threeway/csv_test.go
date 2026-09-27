package threeway

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

func TestCompareAndMergeCSV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, left, right := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	write := func(path, value string) {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(base, "id,name\n1,same\n2,keep-duplicate\n2,base\n3,base-three\n")
	write(left, "id,name\n1,same\n2,keep-duplicate\n2,left\n3,base-three\n4,added\n")
	write(right, "id,name\n1,same\n2,keep-duplicate\n2,right\n3,right-three\n4,added\n")
	cfg := threeWayTestConfig()
	cfg.KeyNames = []string{"id"}
	result, err := CompareCSV(context.Background(), base, left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Conflicts != 1 || result.RightOnly != 1 || result.Same != 1 {
		t.Fatalf("result=%+v events=%+v", result, result.Events)
	}
	choices := map[string]string{}
	for _, event := range result.Events {
		if event.Kind == Conflict {
			choices[event.ID] = "left"
		}
	}
	output := filepath.Join(dir, "merged.csv")
	unresolved, err := WriteCSVMerge(base, output, result, choices, false)
	if err != nil || unresolved != 0 {
		t.Fatalf("unresolved=%d err=%v", unresolved, err)
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(records[1:], func(i, j int) bool { return rowString(records[1:][i]) < rowString(records[1:][j]) })
	want := [][]string{{"id", "name"}, {"1", "same"}, {"2", "keep-duplicate"}, {"2", "left"}, {"3", "right-three"}, {"4", "added"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("records=%#v want=%#v", records, want)
	}
}

// TestWriteCSVMergeTargetSelectsImplicitSide covers #272: an unresolved CSV
// conflict can be sent to left or right instead of always retaining BASE.
func TestWriteCSVMergeTargetSelectsImplicitSide(t *testing.T) {
	dir := t.TempDir()
	base, left, right := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	for path, value := range map[string]string{base: "id,v\n1,b\n", left: "id,v\n1,l\n", right: "id,v\n1,r\n"} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := threeWayTestConfig()
	cfg.KeyNames = []string{"id"}
	result, err := CompareCSV(context.Background(), base, left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{UnresolvedLeft: "l", UnresolvedRight: "r", UnresolvedBase: "b"} {
		// A distinct output path: writing over the input is refused by the
		// server, and on Windows an input held open cannot be replaced (#272).
		output := filepath.Join(dir, "merged-"+target+".csv")
		unresolved, err := WriteCSVMergeTarget(base, output, result, nil, true, target)
		if err != nil || unresolved != 1 {
			t.Fatalf("%s: unresolved=%d err=%v", target, unresolved, err)
		}
		records := readMergedCSV(t, output, "utf-8", ',')
		if len(records) != 2 || records[1][1] != want {
			t.Fatalf("%s: records=%#v want value %q", target, records, want)
		}
	}
	if _, err := WriteCSVMergeTarget(base, filepath.Join(dir, "bad.csv"), result, nil, true, "markers"); err == nil {
		t.Fatal("markers target accepted for CSV")
	}
}

func TestWriteCSVMergeRejectsUnresolved(t *testing.T) {
	dir := t.TempDir()
	base, left, right := filepath.Join(dir, "base.csv"), filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	for path, value := range map[string]string{base: "id,v\n1,b\n", left: "id,v\n1,l\n", right: "id,v\n1,r\n"} {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := threeWayTestConfig()
	cfg.KeyNames = []string{"id"}
	result, err := CompareCSV(context.Background(), base, left, right, cfg)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "rejected.csv")
	if _, err := WriteCSVMerge(base, output, result, nil, false); err == nil {
		t.Fatal("unresolved CSV merge succeeded")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output exists: %v", err)
	}
}

func threeWayTestConfig() engine.Config {
	return engine.Config{HasHeader: true, AlignColumnsByName: true, LeftFormat: "auto", RightFormat: "auto", LeftParser: "auto", RightParser: "auto",
		Partitions: 2, ParseWorkers: 1, Workers: 1, MemoryText: "64MiB", PartitionBufferText: "4KiB", MergeFanIn: 2, MaxRecordText: "1MiB"}
}
