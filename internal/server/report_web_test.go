package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// postReport sends a /api/report body and returns the recorder.
func postReport(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/report", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	return rec
}

func reportFiles(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldPath, []byte("alpha\nSECRET_OLD\nomega\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("alpha\nSECRET_NEW\nomega\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return oldPath, newPath
}

func reportBody(oldPath, newPath, extra string) string {
	body := `{"old":` + jsonString(oldPath) + `,"new":` + jsonString(newPath) +
		`,"mode":"text","window":128,"maxHunks":200,"maxLines":200,"whitespace":"none"` + extra + `}`
	return body
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestReportJSONDescribesTheComparison(t *testing.T) {
	t.Parallel()
	oldPath, newPath := reportFiles(t)
	state := `{"v":1,"mode":"text","paths":{"base":"","old":` + jsonString(oldPath) +
		`,"new":` + jsonString(newPath) + `},"controls":{"ignoreCase":true}}`
	rec := postReport(t, newTestServer(t), reportBody(oldPath, newPath,
		`,"format":"json","readHunks":[0],"ignoredHunks":[0],"comparisonState":`+state))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	var report map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report["generator"] != "ayame-diff" || report["version"] != "test" {
		t.Errorf("generator/version = %v/%v", report["generator"], report["version"])
	}
	inputs, _ := report["inputs"].([]any)
	if len(inputs) != 2 {
		t.Fatalf("inputs = %#v", report["inputs"])
	}
	first, _ := inputs[0].(map[string]any)
	if first["path"] != oldPath || first["size"] == nil || first["mtime"] == nil || first["encoding"] == nil {
		t.Errorf("first input = %#v", first)
	}
	want := sha256.Sum256([]byte("alpha\nSECRET_OLD\nomega\n"))
	if first["sha256"] != hex.EncodeToString(want[:]) {
		t.Errorf("sha256 = %v, want %s", first["sha256"], hex.EncodeToString(want[:]))
	}
	conditions, _ := report["conditions"].(map[string]any)
	if conditions["window"] != float64(128) || conditions["max_hunks"] != float64(200) {
		t.Errorf("conditions = %#v", conditions)
	}
	totals, _ := report["totals"].(map[string]any)
	if totals["modified"] == nil && totals["added"] == nil && totals["deleted"] == nil {
		t.Errorf("totals = %#v", totals)
	}
	ignored, _ := report["ignored_hunks"].([]any)
	if len(ignored) != 1 {
		t.Fatalf("ignored_hunks = %#v", report["ignored_hunks"])
	}
	firstIgnored, _ := ignored[0].(map[string]any)
	if firstIgnored["reason"] == "" {
		t.Errorf("ignored hunk carries no reason: %#v", firstIgnored)
	}
	confirmation, _ := report["confirmation"].(map[string]any)
	if confirmation["available"] != false || confirmation["note"] == "" {
		t.Errorf("confirmation should be unavailable with an honest note: %#v", confirmation)
	}
	reproduce, _ := report["reproduce"].(map[string]any)
	if reproduce["hash_fragment"] == nil || !strings.HasPrefix(reproduce["hash_fragment"].(string), "compare=") {
		t.Errorf("reproduce = %#v", reproduce)
	}
	if report["content_included"] != false {
		t.Errorf("content_included = %v", report["content_included"])
	}
	if strings.Contains(rec.Body.String(), "SECRET_OLD") || strings.Contains(rec.Body.String(), "SECRET_NEW") {
		t.Fatal("report leaked compared content without opt-in")
	}
}

func TestReportDefaultsToNoContentAndNoConfirmedHunks(t *testing.T) {
	t.Parallel()
	oldPath, newPath := reportFiles(t)
	rec := postReport(t, newTestServer(t), reportBody(oldPath, newPath, `,"format":"json"`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET_OLD") {
		t.Fatal("content must not be embedded by default")
	}
	// The read list is available even without confirmation tracking.
	var report map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	review, _ := report["review"].(map[string]any)
	if review["available"] != true {
		t.Errorf("review should still report read hunks: %#v", review)
	}
}

func TestReportHTMLAndMarkdownFormats(t *testing.T) {
	t.Parallel()
	oldPath, newPath := reportFiles(t)
	h := newTestServer(t)

	htmlRec := postReport(t, h, reportBody(oldPath, newPath, `,"format":"html"`))
	if htmlRec.Code != http.StatusOK {
		t.Fatalf("html status = %d body=%s", htmlRec.Code, htmlRec.Body.String())
	}
	if ct := htmlRec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("html Content-Type = %q", ct)
	}
	if !strings.Contains(htmlRec.Body.String(), "@media print") {
		t.Error("html report is not printable")
	}
	if strings.Contains(htmlRec.Body.String(), "SECRET_OLD") {
		t.Fatal("html report leaked content by default")
	}

	mdRec := postReport(t, h, reportBody(oldPath, newPath, `,"format":"markdown"`))
	if mdRec.Code != http.StatusOK {
		t.Fatalf("markdown status = %d body=%s", mdRec.Code, mdRec.Body.String())
	}
	if !strings.Contains(mdRec.Body.String(), "# ayame-diff confirmation report") {
		t.Error("markdown report missing its heading")
	}
}

func TestReportEmbedsContentOnlyWhenOptedIn(t *testing.T) {
	t.Parallel()
	oldPath, newPath := reportFiles(t)
	rec := postReport(t, newTestServer(t), reportBody(oldPath, newPath, `,"format":"json","includeContent":true`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SECRET_OLD") {
		t.Fatal("content should be embedded when explicitly requested")
	}
}

func TestReportRejectsUnknownFormat(t *testing.T) {
	t.Parallel()
	oldPath, newPath := reportFiles(t)
	rec := postReport(t, newTestServer(t), reportBody(oldPath, newPath, `,"format":"pdf"`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReportRejectsUnsupportedMode(t *testing.T) {
	t.Parallel()
	oldPath, newPath := reportFiles(t)
	rec := postReport(t, newTestServer(t), reportBody(oldPath, newPath, `,"mode":"csv"`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestReportExportIsWired guards the UI hookup: the export menu must offer the
// report formats and the explicit content opt-in, and the click must reach the
// API through the mutual-exclusion entry point.
func TestReportExportIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	for _, want := range []string{
		`id="exportReport"`, `id="reportFormat"`, `id="reportIncludeContent"`,
		`data-i18n="reportSensitiveHint"`, `data-i18n="exportReport"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	for _, want := range []string{
		`runExclusive("exportReport", runExportReport)`,
		`/api/report`,
		`body.includeContent = $("reportIncludeContent").checked`,
		`captureComparisonState()`,
		`"exportReport"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}
