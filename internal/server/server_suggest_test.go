package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSuggestFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func postSuggest(t *testing.T, handler http.Handler, body any) (int, []csvSuggestResult) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/csv/suggest", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Results []csvSuggestResult `json:"results"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v (%s)", err, recorder.Body.String())
	}
	return recorder.Code, response.Results
}

// TestCSVSuggestResidualsMatchDiff pins the contract the client depends on: a
// candidate's difference_count is the same number the next Compare would show,
// and applying a normalization lowers it exactly as advertised (#121).
func TestCSVSuggestResidualsMatchDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := writeSuggestFixture(t, dir, "left.csv", "id,name,price\n1,Alice,10.00\n2,Bob,20.00\n3,Carol,30.00\n")
	right := writeSuggestFixture(t, dir, "right.csv", "id,name,price\n1,Alice ,10.00\n2,bob,20.00\n3,Carol,30.01\n")
	handler := newTestServer(t)

	base := csvRequest{Old: left, New: right, HasHeader: true, KeyMode: "include", KeyNames: []string{"id"}}
	whitespace := base
	whitespace.Whitespace = "change"

	tol := 0.01
	partial := whitespace
	partial.IgnoreCase = true
	cleared := partial
	cleared.Tolerance = &tol

	// The number the diff endpoint reports for the base comparison is the
	// ground truth a candidate must not disagree with.
	diffPayload, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	diffRequest := httptest.NewRequest(http.MethodPost, "/api/csv/diff", strings.NewReader(string(diffPayload)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, diffRequest)
	if recorder.Code != http.StatusOK {
		t.Fatalf("diff code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var diffResponse struct {
		DifferenceCount int `json:"difference_count"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &diffResponse); err != nil {
		t.Fatal(err)
	}
	if diffResponse.DifferenceCount != 3 {
		t.Fatalf("fixture difference_count=%d, want 3", diffResponse.DifferenceCount)
	}

	_, results := postSuggest(t, handler, csvSuggestRequest{Candidates: []csvSuggestCandidate{
		{ID: "base", Request: base},
		{ID: "whitespace", Request: whitespace},
		{ID: "partial", Request: partial},
		{ID: "cleared", Request: cleared},
	}})
	want := map[string]int{"base": 3, "whitespace": 2, "partial": 1, "cleared": 0}
	for _, result := range results {
		if result.Error != "" {
			t.Fatalf("candidate %s failed: %s", result.ID, result.Error)
		}
		if result.DifferenceCount != want[result.ID] {
			t.Errorf("candidate %s residual=%d, want %d (summary=%+v)", result.ID, result.DifferenceCount, want[result.ID], result.Summary)
		}
	}
}

// TestCSVSuggestIgnoresConcentratedColumn checks that dropping the column most
// changes sit in is reported as the fix it is.
func TestCSVSuggestIgnoresConcentratedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := writeSuggestFixture(t, dir, "left.csv", "id,value,updated_at\n1,a,t1\n2,b,t2\n")
	right := writeSuggestFixture(t, dir, "right.csv", "id,value,updated_at\n1,a,t1x\n2,b,t2x\n")
	handler := newTestServer(t)

	base := csvRequest{Old: left, New: right, HasHeader: true, KeyMode: "include", KeyNames: []string{"id"}}
	ignored := base
	ignored.IgnoreColumnNames = []string{"updated_at"}
	_, results := postSuggest(t, handler, csvSuggestRequest{Candidates: []csvSuggestCandidate{
		{ID: "base", Request: base},
		{ID: "ignore", Request: ignored},
	}})
	if results[0].DifferenceCount != 2 {
		t.Fatalf("base residual=%d, want 2", results[0].DifferenceCount)
	}
	if results[1].DifferenceCount != 0 {
		t.Fatalf("ignoring updated_at left %d differences, want 0 (%+v)", results[1].DifferenceCount, results[1].Summary)
	}
}

func TestCSVSuggestBoundsAndPerCandidateErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	left := writeSuggestFixture(t, dir, "left.csv", "id,v\n1,a\n")
	right := writeSuggestFixture(t, dir, "right.csv", "id,v\n1,b\n")
	handler := newTestServer(t)
	base := csvRequest{Old: left, New: right, HasHeader: true}

	// Too many candidates is rejected before any engine run.
	candidates := make([]csvSuggestCandidate, 0, maxSuggestCandidates+1)
	for i := 0; i <= maxSuggestCandidates; i++ {
		candidates = append(candidates, csvSuggestCandidate{ID: fmt.Sprintf("c%d", i), Request: base})
	}
	payload, err := json.Marshal(csvSuggestRequest{Candidates: candidates})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/csv/suggest", strings.NewReader(string(payload)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("over-limit request code=%d, want 400", recorder.Code)
	}

	// A malformed candidate becomes a per-result error, not a failed request, so
	// one bad proposal does not hide the residual counts of the others.
	missing := csvRequest{HasHeader: true}
	noKeys := csvRequest{Old: left, New: right, HasHeader: true, KeyMode: "include"}
	_, results := postSuggest(t, handler, csvSuggestRequest{Candidates: []csvSuggestCandidate{
		{ID: "missing", Request: missing},
		{ID: "no-keys", Request: noKeys},
		{ID: "ok", Request: base},
	}})
	if results[0].Error == "" || results[1].Error == "" {
		t.Errorf("malformed candidates should carry an error: %+v", results)
	}
	if results[2].Error != "" {
		t.Errorf("valid candidate failed: %s", results[2].Error)
	}
}

func TestCSVSuggestRequiresPost(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodGet, "/api/csv/suggest", nil)
	recorder := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d, want 405", recorder.Code)
	}
}

// TestSuggestModuleIsWired guards the #121 split: the reasoning the node tests
// check must be the same module the page loads and consumes, and it must stay
// free of DOM/application wiring.
func TestSuggestModuleIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `<script src="suggest.js"></script>`) {
		t.Error("index.html does not load suggest.js")
	}
	if strings.Index(index, `src="suggest.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("suggest.js must load before app.js, which destructures it at parse time")
	}
	module := readWebAsset(t, "suggest.js")
	if !strings.Contains(module, "module.exports = api") {
		t.Error("suggest.js has no CommonJS export, so node --test cannot require it")
	}
	for _, leaked := range []string{"document.", "$(", "addEventListener", "localStorage"} {
		if strings.Contains(module, leaked) {
			t.Errorf("suggest.js contains application wiring (%q); it must stay runnable without a DOM", leaked)
		}
	}
	if app := readWebAsset(t, "app.js"); !strings.Contains(app, "globalThis.AyameSuggest") {
		t.Error("app.js does not consume the extracted suggestion module")
	}
}
