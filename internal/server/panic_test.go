package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRecoveredTurnsAPanicIntoA500 is the #137 regression. Without the
// middleware net/http closes the connection with no body, so the GUI waits on a
// request that never answers and the user sees a hang rather than a failure.
func TestRecoveredTurnsAPanicIntoA500(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(nil) })

	handler := recovered(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("handler exploded on /secret/path.csv")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/diff", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusInternalServerError)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the standard JSON error shape: %v (%q)", err, recorder.Body.String())
	}
	if body["error"] == "" {
		t.Fatalf("body=%v must carry an error message", body)
	}
	// The panic value can carry paths or input fragments and the user can do
	// nothing with it; the log keeps the diagnosable copy.
	if strings.Contains(recorder.Body.String(), "/secret/path.csv") {
		t.Errorf("response leaks the panic detail: %q", recorder.Body.String())
	}
}

// TestRecoveredKeepsServingAfterAPanic is the point of the middleware: one bad
// request must not end the session.
func TestRecoveredKeepsServingAfterAPanic(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(nil) })

	explode := true
	handler := recovered(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if explode {
			panic("boom")
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/diff", nil))
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first status=%d want 500", first.Code)
	}
	explode = false
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/diff", nil))
	if second.Code != http.StatusOK {
		t.Fatalf("second status=%d want 200 — the server stopped working after a panic", second.Code)
	}
}

// TestRecoveredRepanicsOnAbortHandler keeps net/http's documented escape hatch
// working: ErrAbortHandler means "drop this response", and rewriting it as a
// 500 would corrupt an already-started reply.
func TestRecoveredRepanicsOnAbortHandler(t *testing.T) {
	handler := recovered(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if value := recover(); value != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want ErrAbortHandler to propagate", value)
		}
	}()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/diff", nil))
}
