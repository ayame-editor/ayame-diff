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
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

func TestHealth(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	newTestServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "test" {
		t.Fatalf("health = %v", body)
	}
}

func TestIndexServed(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newTestServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ayame-diff") {
		t.Fatal("index.html not served")
	}
}

func TestDiffAPI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldP := filepath.Join(dir, "old.txt")
	newP := filepath.Join(dir, "new.txt")
	os.WriteFile(oldP, []byte("apple\nbanana\ncherry\n"), 0o644)
	os.WriteFile(newP, []byte("apple\nblueberry\ncherry\ndate\n"), 0o644)

	reqBody, _ := json.Marshal(diffRequest{Old: oldP, New: newP, Mode: "text"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(reqBody))
	newTestServer(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp diffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OldLines != 3 || resp.NewLines != 4 {
		t.Fatalf("lines = %d/%d", resp.OldLines, resp.NewLines)
	}
	if resp.Modified != 1 || resp.Added != 1 {
		t.Fatalf("stats: modified=%d added=%d", resp.Modified, resp.Added)
	}
	// The replace hunk must carry its line text for the frontend.
	if len(resp.Hunks) == 0 || resp.Hunks[0].Kind != "replace" ||
		len(resp.Hunks[0].Old) != 1 || resp.Hunks[0].Old[0] != "banana" ||
		resp.Hunks[0].New[0] != "blueberry" {
		t.Fatalf("first hunk = %+v", resp.Hunks)
	}
}

func TestDiffAPIErrors(t *testing.T) {
	t.Parallel()
	h := newTestServer(t)

	// GET is rejected.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/diff", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", rec.Code)
	}

	// Missing paths.
	rec = httptest.NewRecorder()
	body, _ := json.Marshal(diffRequest{Old: "", New: ""})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing-paths status = %d", rec.Code)
	}

	// Nonexistent file.
	rec = httptest.NewRecorder()
	body, _ = json.Marshal(diffRequest{Old: "/no/such/a", New: "/no/such/b"})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing-file status = %d", rec.Code)
	}
}
