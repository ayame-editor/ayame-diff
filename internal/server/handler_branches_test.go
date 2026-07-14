package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHandleDropGuards covers #141: /api/drop's path-traversal and method
// guards (the highest-risk handler, security-relevant to #108/#109) must reject
// every unsafe session/relative combination before touching the filesystem.
func TestHandleDropGuards(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	do := func(method, query string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/drop?"+query, strings.NewReader("x")))
		return rec.Code
	}
	// t.TempDir() is absolute on every OS (unlike a hardcoded "/etc/passwd",
	// which filepath.IsAbs rejects on Windows for lacking a drive letter).
	absolute := t.TempDir()
	for _, c := range []struct {
		name   string
		method string
		query  string
		want   int
	}{
		{"non-post", http.MethodGet, "session=s&relative=f.txt", http.StatusMethodNotAllowed},
		{"empty session", http.MethodPost, "session=&relative=f.txt", http.StatusBadRequest},
		{"session with slash", http.MethodPost, "session=a/b&relative=f.txt", http.StatusBadRequest},
		{"session with backslash", http.MethodPost, "session=a%5Cb&relative=f.txt", http.StatusBadRequest},
		{"empty relative", http.MethodPost, "session=s&relative=", http.StatusBadRequest},
		{"dotdot relative", http.MethodPost, "session=s&relative=..", http.StatusBadRequest},
		{"escaping relative", http.MethodPost, "session=s&relative=" + url.QueryEscape("../secret"), http.StatusBadRequest},
		{"absolute relative", http.MethodPost, "session=s&relative=" + url.QueryEscape(absolute), http.StatusBadRequest},
		{"cleans to escape", http.MethodPost, "session=s&relative=" + url.QueryEscape("a/../../x"), http.StatusBadRequest},
	} {
		if got := do(c.method, c.query); got != c.want {
			t.Errorf("%s: code=%d, want %d", c.name, got, c.want)
		}
	}
}

// TestHandleDropWritesFileAndDirectory covers the two success branches: a file
// upload lands under the session root, and directory=1 creates a directory.
func TestHandleDropWritesFileAndDirectory(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // isolate drop storage on Linux
	h := newTestServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/drop?session=sess1&relative=up.txt", strings.NewReader("payload")))
	if rec.Code != http.StatusOK {
		t.Fatalf("file drop: code=%d body=%s", rec.Code, rec.Body)
	}
	var got struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(got.Path); err != nil || string(data) != "payload" {
		t.Fatalf("uploaded file: data=%q err=%v", data, err)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/drop?session=sess1&relative=sub&directory=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("directory drop: code=%d body=%s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if info, err := os.Stat(got.Path); err != nil || !info.IsDir() {
		t.Fatalf("directory not created: err=%v", err)
	}
}

// TestHandleHealthAndPathInfoBranches covers #141: method rejection and the
// missing/empty-path branches that had no coverage.
func TestHandleHealthAndPathInfoBranches(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	send := func(method, target string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec.Code, rec.Body.String()
	}

	if code, _ := send(http.MethodPost, "/api/health"); code != http.StatusMethodNotAllowed {
		t.Errorf("health non-GET: code=%d, want 405", code)
	}
	if code, _ := send(http.MethodPost, "/api/path-info?path=/tmp"); code != http.StatusMethodNotAllowed {
		t.Errorf("path-info non-GET: code=%d, want 405", code)
	}
	if code, _ := send(http.MethodGet, "/api/path-info"); code != http.StatusBadRequest {
		t.Errorf("path-info empty path: code=%d, want 400", code)
	}
	missing := filepath.Join(t.TempDir(), "nope")
	if code, _ := send(http.MethodGet, "/api/path-info?path="+url.QueryEscape(missing)); code != http.StatusNotFound {
		t.Errorf("path-info missing: code=%d, want 404", code)
	}
	dir := t.TempDir()
	code, body := send(http.MethodGet, "/api/path-info?path="+url.QueryEscape(dir))
	if code != http.StatusOK || !strings.Contains(body, `"directory":true`) {
		t.Errorf("path-info dir: code=%d body=%s", code, body)
	}
}

// TestHandleTextMergeRejections covers #141: the merge handler's guard branches
// (method, malformed JSON, sorted mode, missing paths, invalid choices).
func TestHandleTextMergeRejections(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	post := func(body string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/merge/text", bytes.NewReader([]byte(body))))
		return rec.Code
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/merge/text", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("non-POST: code=%d, want 405", rec.Code)
	}
	if code := post("{not json"); code != http.StatusBadRequest {
		t.Errorf("malformed JSON: code=%d, want 400", code)
	}
	if code := post(`{"mode":"sorted","old":"a","new":"b"}`); code != http.StatusBadRequest {
		t.Errorf("sorted mode: code=%d, want 400", code)
	}
	if code := post(`{"old":"","new":""}`); code != http.StatusBadRequest {
		t.Errorf("missing paths: code=%d, want 400", code)
	}
	// Inline text with an invalid defaultChoice exercises the choice guard.
	if code := post(`{"inline":true,"oldText":"a\n","newText":"b\n","defaultChoice":"middle"}`); code != http.StatusBadRequest {
		t.Errorf("invalid defaultChoice: code=%d, want 400", code)
	}
}

// TestHandleCSVDiffRejections covers #141: decodeCSVRequest / validateCSVKeys
// rejection branches (method, malformed JSON, missing paths, missing keys),
// each of which returns before any file is opened.
func TestHandleCSVDiffRejections(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)
	post := func(body string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/csv/diff", bytes.NewReader([]byte(body))))
		return rec.Code
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/csv/diff", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("non-POST: code=%d, want 405", rec.Code)
	}
	if code := post("{bad"); code != http.StatusBadRequest {
		t.Errorf("malformed JSON: code=%d, want 400", code)
	}
	if code := post(`{"old":"","new":"b"}`); code != http.StatusBadRequest {
		t.Errorf("missing path: code=%d, want 400", code)
	}
	// Valid paths but keyMode=include with no keys is rejected before opening.
	if code := post(`{"old":"a","new":"b","keyMode":"include"}`); code != http.StatusBadRequest {
		t.Errorf("include mode with no keys: code=%d, want 400", code)
	}
}
