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
