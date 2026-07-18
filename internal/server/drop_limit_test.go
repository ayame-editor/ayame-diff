package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestDropLimitsAndCleanup covers #109: each streamed file and each session are
// bounded, errors say which boundary was hit, and an oversized staged upload
// leaves neither a partial target nor a temporary sibling behind.
func TestDropLimitsAndCleanup(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	savedUpload, savedSession := maxDropUploadBytes, maxDropSessionBytes
	maxDropUploadBytes, maxDropSessionBytes = 8, 12
	defer func() {
		maxDropUploadBytes, maxDropSessionBytes = savedUpload, savedSession
	}()

	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	h := authorizedHandler(s)
	drop := func(relative, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost,
			"/api/drop?session=limits&relative="+relative, strings.NewReader(body))
		// Exercise the streaming guard instead of the Content-Length fast path.
		req.ContentLength = -1
		h.ServeHTTP(rec, req)
		return rec
	}

	tooLarge := drop("oversized.txt", "123456789")
	assertDropLimitError(t, tooLarge, "per-file limit")

	knownLength := httptest.NewRecorder()
	h.ServeHTTP(knownLength, httptest.NewRequest(http.MethodPost,
		"/api/drop?session=limits&relative=known-oversized.txt",
		strings.NewReader("123456789")))
	assertDropLimitError(t, knownLength, "per-file limit")

	first := drop("first.txt", "12345678")
	if first.Code != http.StatusOK {
		t.Fatalf("exact per-file limit: code=%d body=%s", first.Code, first.Body)
	}
	var firstResult struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstResult); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(firstResult.Path)
	if _, err := os.Stat(filepath.Join(root, "oversized.txt")); !os.IsNotExist(err) {
		t.Fatalf("oversized target was not cleaned up: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "known-oversized.txt")); !os.IsNotExist(err) {
		t.Fatalf("known-length oversized target was created: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".ayame-drop-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staged upload leaked: matches=%v err=%v", matches, err)
	}

	second := drop("second.txt", "1234")
	if second.Code != http.StatusOK {
		t.Fatalf("exact session limit: code=%d body=%s", second.Code, second.Body)
	}
	sessionOver := drop("third.txt", "x")
	assertDropLimitError(t, sessionOver, "session")

	// Replacing a file accounts for its old size, and a failed replacement
	// preserves the complete old file.
	replaced := drop("first.txt", "xy")
	if replaced.Code != http.StatusOK {
		t.Fatalf("smaller replacement: code=%d body=%s", replaced.Code, replaced.Body)
	}
	failedReplacement := drop("first.txt", "123456789")
	assertDropLimitError(t, failedReplacement, "per-file limit")
	data, err := os.ReadFile(firstResult.Path)
	if err != nil || string(data) != "xy" {
		t.Fatalf("failed replacement changed target: data=%q err=%v", data, err)
	}
}

// TestDropSessionLimitIsRaceSafe verifies concurrent requests for one session
// cannot both observe the same remaining capacity and overfill it.
func TestDropSessionLimitIsRaceSafe(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	savedUpload, savedSession := maxDropUploadBytes, maxDropSessionBytes
	maxDropUploadBytes, maxDropSessionBytes = 8, 10
	defer func() {
		maxDropUploadBytes, maxDropSessionBytes = savedUpload, savedSession
	}()

	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	h := authorizedHandler(s)
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, relative := range []string{"a.txt", "b.txt"} {
		wg.Add(1)
		go func(relative string) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
				"/api/drop?session=concurrent&relative="+relative,
				strings.NewReader("12345678")))
			codes <- rec.Code
		}(relative)
	}
	wg.Wait()
	close(codes)

	ok, tooLarge := 0, 0
	for code := range codes {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusRequestEntityTooLarge:
			tooLarge++
		default:
			t.Fatalf("unexpected status %d", code)
		}
	}
	if ok != 1 || tooLarge != 1 {
		t.Fatalf("statuses: ok=%d tooLarge=%d, want 1/1", ok, tooLarge)
	}
}

func assertDropLimitError(t *testing.T, rec *httptest.ResponseRecorder, contains string) {
	t.Helper()
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code=%d, want 413; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), contains) {
		t.Fatalf("body=%q, want %q", rec.Body.String(), contains)
	}
}
