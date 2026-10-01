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

// TestCSVDiffVerdictClassifiesPresentationOnly is the #116 server check: the
// response must state whether the inputs are substantively the same apart from
// column or row order, and offer to align columns when that is all that stands
// in the way of a comparison.
func TestCSVDiffVerdictClassifiesPresentationOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		left, right   string
		align         bool
		keyNames      []string
		wantStatus    engine.VerdictStatus
		wantEqual     bool
		wantDiffs     uint64
		wantAlignable bool
	}{
		{
			name: "columns reordered without alignment offers a fix",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "value,id,name\n10,1,a\n20,2,b\n",
			align: false, wantStatus: engine.VerdictColumnsReordered, wantEqual: false, wantDiffs: 0, wantAlignable: true,
		},
		{
			name: "columns reordered with alignment is data equal",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "value,id,name\n10,1,a\n20,2,b\n",
			align: true, wantStatus: engine.VerdictEqualColumns, wantEqual: true, wantDiffs: 0, wantAlignable: true,
		},
		{
			name: "rows reordered is data equal",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "id,name,value\n2,b,20\n1,a,10\n",
			align: true, wantStatus: engine.VerdictEqualRows, wantEqual: true, wantDiffs: 0,
		},
		{
			name: "real difference is reported as a count",
			left: "id,name,value\n1,a,10\n2,b,20\n", right: "id,name,value\n1,a,10\n",
			align: true, keyNames: []string{"id"}, wantStatus: engine.VerdictDifferences, wantEqual: false, wantDiffs: 1,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			left, right := filepath.Join(dir, "left.csv"), filepath.Join(dir, "right.csv")
			if err := os.WriteFile(left, []byte(tt.left), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(right, []byte(tt.right), 0o600); err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"old": left, "new": right, "hasHeader": true, "alignColumnsByName": tt.align}
			if len(tt.keyNames) > 0 {
				body["keyMode"], body["keyNames"] = "include", tt.keyNames
			}
			payload, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/csv/diff", strings.NewReader(string(payload)))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			newTestServer(t).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("code=%d body=%s", recorder.Code, recorder.Body.String())
			}
			var response csvResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode: %v (%s)", err, recorder.Body.String())
			}
			got := response.Verdict
			if got.Status != tt.wantStatus || got.SubstantivelyEqual != tt.wantEqual ||
				got.Differences != tt.wantDiffs || got.AlignableColumns != tt.wantAlignable {
				t.Fatalf("verdict = %+v; want status=%q equal=%v diffs=%d alignable=%v",
					got, tt.wantStatus, tt.wantEqual, tt.wantDiffs, tt.wantAlignable)
			}
		})
	}
}

// TestEquivalenceModuleIsWiredUp keeps the verdict mapping in the module the
// page actually loads and the node tests actually require (#116, #139).
func TestEquivalenceModuleIsWiredUp(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "equivalence.js")

	if !strings.Contains(index, `<script src="equivalence.js"></script>`) {
		t.Fatal("index.html does not load equivalence.js")
	}
	if strings.Index(index, `src="equivalence.js"`) > strings.Index(index, `src="app.js"`) {
		t.Fatal("equivalence.js loads after app.js, which consumes it")
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Fatal("equivalence.js has no CommonJS export")
	}
	if strings.Contains(module, "document.") {
		t.Fatal("equivalence.js touches the DOM; it must stay runnable without one")
	}
	if !strings.Contains(app, "globalThis.AyameEquivalence") {
		t.Fatal("app.js does not consume the equivalence module")
	}
	if !strings.Contains(app, "function csvAlignProposalCard(") {
		t.Fatal("app.js has no one-click column-alignment path")
	}
}
