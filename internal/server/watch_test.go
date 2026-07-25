package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func postWatchRequest(handler http.Handler, request watchRequest) (int, watchResponse, string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return 0, watchResponse{}, "", err
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/watch", bytes.NewReader(body)))
	var response watchResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			return rec.Code, watchResponse{}, rec.Body.String(), err
		}
	}
	return rec.Code, response, rec.Body.String(), nil
}

func postWatch(t *testing.T, handler http.Handler, request watchRequest) (int, watchResponse, string) {
	t.Helper()
	code, response, body, err := postWatchRequest(handler, request)
	if err != nil {
		t.Fatalf("watch request: %v body=%s", err, body)
	}
	return code, response, body
}

func TestWatchReturnsCanonicalInitialSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	code, response, body := postWatch(t, newTestServer(t), watchRequest{Paths: []string{path, path}})
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if len(response.Changed) != 0 || len(response.Snapshot) != 1 {
		t.Fatalf("response=%+v", response)
	}
	state := response.Snapshot[0]
	if state.Path != path || !state.Exists || state.Directory || state.Size != "7" || state.Modified == "" || state.Mode == "" {
		t.Fatalf("state=%+v", state)
	}
}

func TestWatchDetectsExternalReplacement(t *testing.T) {
	// Serial because this test shortens package-level polling durations.
	savedPoll, savedTimeout := watchPollInterval, watchRequestTimeout
	watchPollInterval, watchRequestTimeout = 5*time.Millisecond, time.Second
	defer func() { watchPollInterval, watchRequestTimeout = savedPoll, savedTimeout }()

	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := newTestServer(t)
	_, initial, _ := postWatch(t, handler, watchRequest{Paths: []string{path}})

	type result struct {
		code     int
		response watchResponse
		body     string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		code, response, body, err := postWatchRequest(handler, watchRequest{
			Paths: []string{path}, Baseline: initial.Snapshot,
		})
		done <- result{code, response, body, err}
	}()
	time.Sleep(25 * time.Millisecond)
	replacement := filepath.Join(filepath.Dir(path), "replacement.txt")
	if err := os.WriteFile(replacement, []byte("after replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("watch request: %v body=%s", got.err, got.body)
		}
		if got.code != http.StatusOK {
			t.Fatalf("status=%d body=%s", got.code, got.body)
		}
		if len(got.response.Changed) != 1 || got.response.Changed[0] != path {
			t.Fatalf("response=%+v", got.response)
		}
		if got.response.Snapshot[0].Size != "18" {
			t.Fatalf("snapshot=%+v", got.response.Snapshot)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not return after the file was replaced")
	}
}

func TestWatchTimeoutRenewsSnapshotWithoutChange(t *testing.T) {
	// Serial because this test shortens the package-level long-poll timeout.
	savedPoll, savedTimeout := watchPollInterval, watchRequestTimeout
	watchPollInterval, watchRequestTimeout = 5*time.Millisecond, 20*time.Millisecond
	defer func() { watchPollInterval, watchRequestTimeout = savedPoll, savedTimeout }()

	path := filepath.Join(t.TempDir(), "stable.txt")
	if err := os.WriteFile(path, []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := newTestServer(t)
	_, initial, _ := postWatch(t, handler, watchRequest{Paths: []string{path}})
	code, response, body := postWatch(t, handler, watchRequest{
		Paths: []string{path}, Baseline: initial.Snapshot,
	})
	if code != http.StatusOK || len(response.Changed) != 0 {
		t.Fatalf("status=%d response=%+v body=%s", code, response, body)
	}
	if len(response.Snapshot) != 1 || response.Snapshot[0] != initial.Snapshot[0] {
		t.Fatalf("snapshot changed without a file change: before=%+v after=%+v", initial.Snapshot, response.Snapshot)
	}
}

func TestWatchRejectsUnsafeOrUnsupportedRequests(t *testing.T) {
	t.Parallel()
	handler := newTestServer(t)
	dir := t.TempDir()
	for _, test := range []struct {
		name    string
		request watchRequest
	}{
		{name: "empty", request: watchRequest{}},
		{name: "too many", request: watchRequest{Paths: []string{"a", "b", "c", "d"}}},
		{name: "directory", request: watchRequest{Paths: []string{dir}}},
		{name: "mismatched baseline", request: watchRequest{
			Paths:    []string{filepath.Join(dir, "missing")},
			Baseline: []watchPathState{{Path: filepath.Join(dir, "another")}},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, _, _ := postWatch(t, handler, test.request)
			if code != http.StatusBadRequest {
				t.Fatalf("status=%d want %d", code, http.StatusBadRequest)
			}
		})
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/watch", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestWatchBoundsConcurrentLongPolls(t *testing.T) {
	t.Parallel()
	server, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < cap(server.watchSem); i++ {
		server.watchSem <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(server.watchSem); i++ {
			<-server.watchSem
		}
	}()

	code, _, body := postWatch(t, authorizedHandler(server), watchRequest{Paths: []string{"bounded.txt"}})
	if code != http.StatusTooManyRequests {
		t.Fatalf("status=%d want %d body=%s", code, http.StatusTooManyRequests, body)
	}
}
