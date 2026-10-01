package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// TestDiffResponseReportsDetectedEncoding covers #130: /api/diff surfaces the
// encoding each file side was decoded from, so `encoding: auto` results reveal
// what was guessed and a left/right mismatch is visible. Inline text reports no
// encoding (it is already UTF-8).
func TestDiffResponseReportsDetectedEncoding(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jp := "日本語の差分テスト\n二行目\n"
	sjis, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(jp))
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldPath, sjis, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(jp), 0o644); err != nil { // UTF-8
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{"old": oldPath, "new": newPath, "encoding": "auto"})
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		OldEncoding string `json:"old_encoding"`
		NewEncoding string `json:"new_encoding"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OldEncoding != "shift_jis" {
		t.Errorf("old_encoding = %q, want shift_jis", resp.OldEncoding)
	}
	if resp.NewEncoding != "utf-8" {
		t.Errorf("new_encoding = %q, want utf-8", resp.NewEncoding)
	}
	if resp.OldEncoding == resp.NewEncoding {
		t.Error("expected the left/right encoding mismatch to be reported")
	}

	// Inline (scratch) text is already UTF-8; the fields are omitted.
	inline, _ := json.Marshal(map[string]any{"inline": true, "oldText": "a\n", "newText": "b\n", "mode": "text"})
	rec2 := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(inline)))
	if rec2.Code != http.StatusOK {
		t.Fatalf("inline status=%d body=%s", rec2.Code, rec2.Body)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec2.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["old_encoding"]; ok {
		t.Error("inline response should omit old_encoding")
	}
}

// TestEncodingDisplayIsWired guards that renderSummary consumes the encoding
// fields and flags a mismatch (#130), without needing a browser.
func TestEncodingDisplayIsWired(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")
	for _, want := range []string{"res.old_encoding", "res.new_encoding", `t("encodingDetected"`, `t("encodingMismatch")`, "encoding-mismatch"} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	if !strings.Contains(style, ".encoding-mismatch") {
		t.Error("style.css missing .encoding-mismatch rule")
	}
}

// TestDiffResponsePerSideEncodingOverride covers #278: detection is per side, so
// a left/right mismatch can be corrected one side at a time. oldEncoding and
// newEncoding override the shared encoding for exactly one side.
func TestDiffResponsePerSideEncodingOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jp := "名前,金額\n田中,1000\n"

	sjis, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(jp))
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "old-sjis.csv")
	newPath := filepath.Join(dir, "new-utf8.csv")
	if err := os.WriteFile(oldPath, sjis, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(jp), 0o644); err != nil { // UTF-8
		t.Fatal(err)
	}

	// The shared hint is deliberately wrong; per-side overrides must win and
	// must be independent of each other.
	body, _ := json.Marshal(map[string]any{
		"old": oldPath, "new": newPath,
		"encoding": "euc-jp", "oldEncoding": "shift_jis", "newEncoding": "utf-8",
	})
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		OldEncoding string `json:"old_encoding"`
		NewEncoding string `json:"new_encoding"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OldEncoding != "shift_jis" {
		t.Errorf("old_encoding = %q, want shift_jis (the per-side override)", resp.OldEncoding)
	}
	if resp.NewEncoding != "utf-8" {
		t.Errorf("new_encoding = %q, want utf-8 (the per-side override)", resp.NewEncoding)
	}

	// Absent overrides fall back to the shared encoding, preserving the old
	// single-select behavior.
	plain, _ := json.Marshal(map[string]any{"old": oldPath, "new": newPath, "encoding": "shift_jis"})
	rec2 := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(plain)))
	if rec2.Code != http.StatusOK {
		t.Fatalf("fallback status=%d body=%s", rec2.Code, rec2.Body)
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OldEncoding != "shift_jis" || resp.NewEncoding != "shift_jis" {
		t.Errorf("fallback encodings = %q/%q, want shift_jis/shift_jis", resp.OldEncoding, resp.NewEncoding)
	}
}

// TestEncodingDropdownMatchesEngine keeps the GUI's selectable encodings and the
// engine's list from drifting apart (#278). The browser's encoding module is
// checked against this same markup by encoding.test.js, so the three stay in
// step.
func TestEncodingDropdownMatchesEngine(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	block := sectionBetween(t, index, `<select id="encoding">`, "</select>")
	pattern := regexp.MustCompile(`<option value="([^"]+)"`)
	var got []string
	for _, match := range pattern.FindAllStringSubmatch(block, -1) {
		got = append(got, match[1])
	}
	if !reflect.DeepEqual(got, encoding.Supported) {
		t.Errorf("encoding dropdown = %v, want %v (internal/encoding.Supported)", got, encoding.Supported)
	}
}

// TestPaneHeaderOffersEncodingCorrection covers #278: the detected encoding is
// visible in the pane header, a left/right mismatch is marked there, and a
// picker re-reads one side under another encoding without leaving the result.
func TestPaneHeaderOffersEncodingCorrection(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	index := readWebAsset(t, "index.html")
	style := readWebAsset(t, "style.css")

	header := renderFunctionBody(t, app, "function paneHeads(")
	for _, want := range []string{
		"pane-head-encoding",
		"encodingCandidates(",
		"setEncodingOverride(",
		"void compare();",
		"encoding-mismatch",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("paneHeads is missing %q", want)
		}
	}
	// The per-side correction reaches the request, and nothing else has to
	// change for the server to honor it.
	request := renderFunctionBody(t, app, "function requestBody(")
	for _, want := range []string{"oldEncoding:", "newEncoding:"} {
		if !strings.Contains(request, want) {
			t.Errorf("requestBody is missing %q", want)
		}
	}
	if !strings.Contains(index, `<script src="encoding.js"></script>`) {
		t.Error("index.html does not load encoding.js")
	}
	for _, want := range []string{".pane-head-encoding {", ".pane-head-meta.encoding-mismatch"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css is missing %q", want)
		}
	}
}
