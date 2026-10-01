package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

// TestCSVDiffManualColumnMapAPI exercises the endpoint a hand-built mapping
// reaches (#119): two files with different column names and order compare
// equal once the user states which left column is which right column.
func TestCSVDiffManualColumnMapAPI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,label,amount\n1,alpha,10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("amt,key,tag\n10,1,alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := csvRequest{
		Old: left, New: right, HasHeader: true,
		KeyMode: "include", KeyNames: []string{"id"}, MaxRows: 20,
		ColumnMap: []engine.ColumnPair{{Left: 0, Right: 1}, {Left: 1, Right: 2}, {Left: 2, Right: 0}},
	}
	body, _ := json.Marshal(request)
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/csv/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("diff status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result csvResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Summary.EqualRows != 1 || result.DifferenceCount != 0 || len(result.Differences) != 0 {
		t.Fatalf("result=%+v", result)
	}
	if !reflect.DeepEqual(result.Header, []string{"id", "label", "amount"}) {
		t.Fatalf("header=%#v", result.Header)
	}
}

// TestCSVProjectRoundTripsColumnMap keeps the saved-mapping promise: the map
// chosen on the result survives /api/project/save and /api/project/load.
func TestCSVProjectRoundTripsColumnMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
	if err := os.WriteFile(left, []byte("id,label\n1,alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(right, []byte("tag,key\nalpha,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := csvRequest{
		Old: left, New: right, HasHeader: true, KeyMode: "include", KeyNames: []string{"id"},
		Output: filepath.Join(dir, "export.tsv"), OutputFormat: "tsv",
		ProjectPath: filepath.Join(dir, "mapped.ayamediff.json"),
		ColumnMap:   []engine.ColumnPair{{Left: 0, Right: 1}, {Left: 1, Right: 0, Ignore: true}},
	}
	body, _ := json.Marshal(request)
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/project/save", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	loadBody, _ := json.Marshal(map[string]string{"path": request.ProjectPath})
	rec = httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/project/load", bytes.NewReader(loadBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("load status=%d body=%s", rec.Code, rec.Body.String())
	}
	var loaded csvRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	want := []engine.ColumnPair{{Left: 0, Right: 1}, {Left: 1, Right: 0, Ignore: true}}
	if !reflect.DeepEqual(loaded.ColumnMap, want) {
		t.Fatalf("loaded columnMap=%#v, want %#v", loaded.ColumnMap, want)
	}
}

// TestManualColumnMappingIsWired is the #119 UI check: the page loads the pure
// mapping module before app.js, app.js consumes it, sends columnMap, restores
// it from a project, and the result header is draggable.
func TestManualColumnMappingIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "columnmap.js")

	if !strings.Contains(index, `<script src="columnmap.js"></script>`) {
		t.Error("index.html does not load columnmap.js")
	}
	if strings.Index(index, `src="columnmap.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("columnmap.js must load before app.js, which consumes it")
	}
	if !strings.Contains(index, `id="columnMapRows"`) || !strings.Contains(index, `id="applyColumnMap"`) {
		t.Error("index.html has no manual mapping editor")
	}

	if !strings.Contains(module, "module.exports = api") {
		t.Error("columnmap.js has no CommonJS export, so node --test cannot require it")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "$(") {
		t.Error("columnmap.js touches the DOM; it must stay runnable without one")
	}

	for _, want := range []string{
		"globalThis.AyameColumnMap",
		"function renderColumnMap(",
		"function buildColumnMap(",
		"function applyColumnMap(",
		"function renderCSVHeader(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing mapping wiring %q", want)
		}
	}
	request := renderFunctionBody(t, app, "function csvRequestBody(")
	if !strings.Contains(request, "body.columnMap") {
		t.Error("csvRequestBody does not send columnMap")
	}
	load := renderFunctionBody(t, app, "async function applyCSVProject(")
	if !strings.Contains(load, "setColumnMap(body.columnMap") {
		t.Error("applyCSVProject does not restore a saved columnMap")
	}
	header := renderFunctionBody(t, app, "function renderCSVHeader(")
	if !strings.Contains(header, "th.draggable = true") {
		t.Error("result column headers are not draggable")
	}
	style := readWebAsset(t, "style.css")
	if !strings.Contains(style, ".column-map-row") {
		t.Error("style.css has no mapping row style")
	}
}
