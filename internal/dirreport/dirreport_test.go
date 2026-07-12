package dirreport

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/hjosugi/ayame-diff/internal/dircompare"
)

func reportFixture() *dircompare.Result {
	return &dircompare.Result{
		Added: 1, Removed: 1, Changed: 1, Same: 1,
		Entries: []dircompare.Entry{
			{Path: "added,<script>.txt", Status: dircompare.Added, NewSize: 7},
			{Path: "dir/changed.txt", Status: dircompare.Changed, OldSize: 2, NewSize: 3, OldModTime: time.Unix(1, 0)},
			{Path: "removed.txt", Status: dircompare.Removed, OldSize: 4},
			{Path: "same.txt", Status: dircompare.Same, OldSize: 5, NewSize: 5},
		},
	}
}

func TestWriteHTML(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := WriteHTML(&output, reportFixture(), "old < new", false); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, want := range []string{"<!doctype html>", "1</b> added", "dir/changed.txt", "--depth:1", "old &lt; new", "</body></html>"} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	if strings.Contains(got, "<script>") || strings.Contains(got, "same.txt") {
		t.Fatalf("HTML contains unsafe or filtered content: %s", got)
	}
}

func TestWriteCSV(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := WriteCSV(&output, reportFixture(), true); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(&output).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 5 || records[1][1] != "added,<script>.txt" || records[4][0] != "same" {
		t.Fatalf("records = %#v", records)
	}
}
