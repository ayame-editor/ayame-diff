package engine

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInspectInputsReorderedHeaders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.tsv")
	right := filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id\t名前\tvalue\n1\t東京\tA\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("value,名前,id\nA,東京,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectInputs(Config{
		LeftPath: left, RightPath: right,
		HasHeader: true, AlignColumnsByName: true,
		LeftFormat: "auto", RightFormat: "auto",
		LeftParser: "auto", RightParser: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inspection.Header, []string{"id", "名前", "value"}) {
		t.Fatalf("Header = %#v", inspection.Header)
	}
	if inspection.ColumnCount != 3 || !inspection.ColumnsReordered {
		t.Fatalf("inspection = %#v", inspection)
	}
	if inspection.LeftFormat != "TSV" || inspection.RightFormat != "CSV" {
		t.Fatalf("formats = %q, %q", inspection.LeftFormat, inspection.RightFormat)
	}
}

func TestInspectInputsGzipQuotedMultilineHeader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.csv.gz")
	right := filepath.Join(dir, "right.csv.gz")
	writeGzipFixture(t, left, "\"customer,id\",\"multi\nline\",value\n1,x,A\n")
	writeGzipFixture(t, right, "value,\"multi\nline\",\"customer,id\"\nA,x,1\n")

	inspection, err := InspectInputs(Config{
		LeftPath: left, RightPath: right,
		HasHeader: true, AlignColumnsByName: true,
		LeftFormat: "auto", RightFormat: "auto",
		LeftParser: "auto", RightParser: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"customer,id", "multi\nline", "value"}
	if !reflect.DeepEqual(inspection.Header, want) {
		t.Fatalf("Header = %#v, want %#v", inspection.Header, want)
	}
	if inspection.LeftFormat != "CSV (gzip)" || inspection.RightFormat != "CSV (gzip)" {
		t.Fatalf("formats = %q, %q", inspection.LeftFormat, inspection.RightFormat)
	}
	if inspection.LeftParser != "RFC 4180" || inspection.RightParser != "RFC 4180" {
		t.Fatalf("parsers = %q, %q", inspection.LeftParser, inspection.RightParser)
	}
}

func writeGzipFixture(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(file)
	if _, err := writer.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInspectInputsHeaderlessSyntheticNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.tsv")
	right := filepath.Join(dir, "right.tsv")
	for _, path := range []string{left, right} {
		if err := os.WriteFile(path, []byte("1\t2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inspection, err := InspectInputs(Config{
		LeftPath: left, RightPath: right,
		HasHeader:  false,
		LeftFormat: "auto", RightFormat: "auto",
		LeftParser: "auto", RightParser: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inspection.Header, []string{"column_0", "column_1"}) {
		t.Fatalf("Header = %#v", inspection.Header)
	}
}
