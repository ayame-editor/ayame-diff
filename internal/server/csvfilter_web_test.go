package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

func filterCond(column, op, value string) engine.RowCondition {
	return engine.RowCondition{Column: column, Op: op, Value: value}
}

// TestCSVRequestMapsRowAndColumnFilters guards the server seam: a web request
// carries the builder's trees as JSON and must hand them to the engine on the
// same struct fields the project file serializes.
func TestCSVRequestMapsRowAndColumnFilters(t *testing.T) {
	t.Parallel()
	req := csvRequest{
		Old: "left.csv", New: "right.csv", Output: "out.tsv",
		RowFilter:    &engine.RowFilter{Conditions: []engine.RowCondition{filterCond("status", "eq", "active")}},
		ColumnFilter: &engine.RowFilter{Conditions: []engine.RowCondition{filterCond("", "starts", "tmp_")}},
	}
	cfg := csvConfig(req, "out.tsv")
	if cfg.RowFilter == nil || len(cfg.RowFilter.Conditions) != 1 || cfg.RowFilter.Conditions[0].Column != "status" {
		t.Fatalf("row filter did not reach the engine config: %#v", cfg.RowFilter)
	}
	if cfg.ColumnFilter == nil || cfg.ColumnFilter.Conditions[0].Value != "tmp_" {
		t.Fatalf("column filter did not reach the engine config: %#v", cfg.ColumnFilter)
	}
	roundTrip := requestFromConfig(cfg)
	if roundTrip.RowFilter == nil || roundTrip.ColumnFilter == nil {
		t.Fatalf("filters did not survive the config round trip: %#v", roundTrip)
	}
}

func TestCSVPreviewEndpointReportsFilteredCounts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,region,tmp_x,value\n1,east,a,10\n2,west,b,20\n3,east,c,30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,region,tmp_x,value\n1,east,a,11\n2,west,b,20\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"old": left, "new": right, "hasHeader": true, "alignColumnsByName": true,
		"keyMode":      "all",
		"rowFilter":    map[string]any{"match": "all", "conditions": []map[string]any{{"column": "region", "op": "eq", "value": "east"}}},
		"columnFilter": map[string]any{"conditions": []map[string]any{{"op": "starts", "value": "tmp_"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/csv/preview", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var preview engine.FilterPreview
	if err := json.Unmarshal(recorder.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.LeftTotal != 3 || preview.LeftMatched != 2 {
		t.Fatalf("left = %d/%d, want 2/3", preview.LeftMatched, preview.LeftTotal)
	}
	if preview.RightTotal != 2 || preview.RightMatched != 1 {
		t.Fatalf("right = %d/%d, want 1/2", preview.RightMatched, preview.RightTotal)
	}
	if preview.KeptColumns != 3 || len(preview.IgnoredColumns) != 1 || preview.IgnoredColumns[0] != "tmp_x" {
		t.Fatalf("columns = kept %d ignored %#v", preview.KeptColumns, preview.IgnoredColumns)
	}
}

func TestCSVDiffEndpointAppliesRowFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,region,value\n1,east,10\n2,west,20\n3,east,30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,region,value\n1,east,11\n2,west,20\n3,east,30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"old": left, "new": right, "hasHeader": true, "alignColumnsByName": true,
		"keyNames": []string{"id"}, "keyMode": "include",
		"rowFilter": map[string]any{"conditions": []map[string]any{{"column": "region", "op": "eq", "value": "east"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/csv/diff", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Summary engine.Summary `json:"summary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Summary.LeftRows != 2 || response.Summary.RightRows != 2 {
		t.Fatalf("filtered rows = %d/%d, want 2/2", response.Summary.LeftRows, response.Summary.RightRows)
	}
}

func TestCSVPreviewRejectsUnknownColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,value\n1,a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("id,value\n1,a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"old": left, "new": right, "hasHeader": true,
		"rowFilter": map[string]any{"conditions": []map[string]any{{"column": "missing", "op": "eq", "value": "x"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/csv/preview", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

// TestFilterBuilderIsWiredUp keeps the tested module in step with the page:
// rowfilter.js loads before app.js, the page has the builder markup, and app.js
// consumes the module instead of reimplementing the compiler.
func TestFilterBuilderIsWiredUp(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `<script src="rowfilter.js"></script>`) {
		t.Fatal("index.html does not load rowfilter.js")
	}
	if strings.Index(index, `src="rowfilter.js"`) > strings.Index(index, `src="app.js"`) {
		t.Fatal("rowfilter.js loads after app.js, which consumes it")
	}
	module := readWebAsset(t, "rowfilter.js")
	if !strings.Contains(module, "module.exports = api") {
		t.Fatal("rowfilter.js has no CommonJS export")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "$(") {
		t.Error("rowfilter.js touches the DOM; it must stay runnable under node")
	}
	app := readWebAsset(t, "app.js")
	for _, want := range []string{
		"globalThis.AyameRowFilter",
		"function renderFilterBuilders(",
		"function readFilterGroup(",
		"function runFilterPreview(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing %q", want)
		}
	}
	for _, want := range []string{
		`id="csvFilterBuilder"`,
		`data-filter-target="rows"`,
		`data-filter-target="columns"`,
		`id="filterPreview"`,
		`id="filterPreviewResult"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html is missing %q", want)
		}
	}
}
