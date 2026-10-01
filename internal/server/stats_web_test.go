package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

// TestCSVDiffReturnsPerColumnStatistics pins the #120 response contract: the
// CSV summary carries every changed column, each with its share of the changed
// rows, and numeric columns carry a right-minus-left delta summary.
func TestCSVDiffReturnsPerColumnStatistics(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,name,price\n1,alpha,10\n2,beta,20\n3,gamma,30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,name,price\n1,alpha,12.5\n2,beta,25\n3,delta,30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := csvRequest{Old: left, New: right, HasHeader: true, AlignColumnsByName: true, KeyMode: "include", KeyNames: []string{"id"}}
	body, _ := json.Marshal(request)
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/csv/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result csvResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Summary.ChangedRows != 3 {
		t.Fatalf("changed_rows=%d, want 3", result.Summary.ChangedRows)
	}
	byName := map[string]engine.ColumnChange{}
	for _, column := range result.Summary.ColumnChanges {
		byName[column.Name] = column
	}
	// price changes on rows 1 (+2.5) and 2 (+5); name on row 3.
	if price := byName["price"]; price.Count != 2 || price.Numeric == nil || price.Numeric.Sum != 7.5 ||
		price.Numeric.Increased != 2 || price.Share != 2.0/3.0 {
		t.Fatalf("price stats=%+v", price)
	}
	if name := byName["name"]; name.Count != 1 || name.Numeric != nil || name.Share != 1.0/3.0 {
		t.Fatalf("name stats=%+v", name)
	}
	if len(result.Summary.ColumnChanges) != 2 {
		t.Fatalf("columns=%+v, the response must not cap the breakdown", result.Summary.ColumnChanges)
	}
}

// TestTextDiffReturnsWholeFileContext covers the text half of #120: the diff
// response reports the share of the file that changed and the largest hunk for
// the whole file, not just the hunks that fit under maxHunks.
func TestTextDiffReturnsWholeFileContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := writeFile(t, dir, "old.txt", []byte("a\nb\nf\ng\nh\ni\nj\n"))
	// Insert c/d/e between b and f: one 3-line hunk, 3 changed lines out of a
	// 10-line file.
	newPath := writeFile(t, dir, "new.txt", []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"))
	rec, resp := postDiff(t, newTestServer(t), diffRequest{Old: old, New: newPath})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if resp.ChangedLines != 3 {
		t.Fatalf("changed_lines=%d, want 3", resp.ChangedLines)
	}
	if resp.LargestHunk != 3 {
		t.Fatalf("largest_hunk=%d, want 3", resp.LargestHunk)
	}
	if want := 0.3; resp.ChangedShare < want-1e-9 || resp.ChangedShare > want+1e-9 {
		t.Fatalf("changed_share=%v, want %v", resp.ChangedShare, want)
	}
}

// TestStatsViewIsWiredUp guards the #120 extraction the same way the other
// pure modules are guarded: the page loads the tested copy, the module stays
// DOM-free, and the renderer no longer caps the column list.
func TestStatsViewIsWiredUp(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "statsview.js")

	if !strings.Contains(index, `<script src="statsview.js"></script>`) {
		t.Error("index.html does not load statsview.js")
	}
	if strings.Index(index, `src="statsview.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("statsview.js must load before app.js, which consumes it")
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("statsview.js has no CommonJS export, so node --test cannot require it")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "$(") {
		t.Error("statsview.js touches the DOM; it must stay runnable without one")
	}
	if !strings.Contains(app, "globalThis.AyameStatsView") {
		t.Error("app.js does not consume the extracted statistics module")
	}
	if strings.Contains(app, "column_changes || []).slice(0, 8)") {
		t.Error("the CSV column breakdown is still capped at 8")
	}
	for _, call := range []string{`statsPanel("csv", data)`, `statsPanel("text", data)`} {
		if !strings.Contains(app, call) {
			t.Errorf("app.js does not render the statistics panel: missing %q", call)
		}
	}
	style := readWebAsset(t, "style.css")
	for _, rule := range []string{".stats-view", ".stats-scroll", ".stat-bar-fill"} {
		if !strings.Contains(style, rule) {
			t.Errorf("style.css is missing %q", rule)
		}
	}
}
