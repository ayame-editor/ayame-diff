package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/engine"
)

func TestClassifyErrorNamesTheFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		fallback int
		code     string
		status   int
	}{
		{"missing file", &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrNotExist}, http.StatusBadRequest, CodeFileNotFound, http.StatusNotFound},
		{"denied", &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}, http.StatusBadRequest, CodePermissionDenied, http.StatusForbidden},
		{"cancelled", context.Canceled, http.StatusBadRequest, CodeTimeout, http.StatusRequestTimeout},
		{"deadline", context.DeadlineExceeded, http.StatusInternalServerError, CodeTimeout, http.StatusRequestTimeout},
		{"unresolved rows", engine.ErrUnresolvedRows, http.StatusInternalServerError, CodeUnsupportedInput, http.StatusBadRequest},
		{"other bad request", errors.New("nope"), http.StatusBadRequest, CodeInvalidRequest, http.StatusBadRequest},
		{"other failure", errors.New("nope"), http.StatusInternalServerError, CodeInternal, http.StatusInternalServerError},
		{"unclassified status", errors.New("nope"), http.StatusTeapot, "", http.StatusTeapot},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			code, status := classifyError(test.err, test.fallback)
			if code != test.code || status != test.status {
				t.Fatalf("code=%q status=%d, want %q and %d", code, status, test.code, test.status)
			}
		})
	}
}

func TestSideErrorKeepsItsMessageAndReportsTheSide(t *testing.T) {
	t.Parallel()

	err := rightError(&fs.PathError{Op: "open", Path: "/data/right.csv", Err: fs.ErrNotExist})
	if got, want := err.Error(), "right: open /data/right.csv: file does not exist"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the wrapped cause is no longer reachable")
	}
	path, side := errorDetail(err)
	if path != "/data/right.csv" || side != "right" {
		t.Fatalf("path=%q side=%q", path, side)
	}
	if path, side := errorDetail(errors.New("plain")); path != "" || side != "" {
		t.Fatalf("a plain error reported path=%q side=%q", path, side)
	}
}

// The browser needs the code and the failing path as data; parsing them back
// out of the English sentence is exactly what #94 removes.
func TestDiffFailureAnswersWithCodeAndPath(t *testing.T) {
	t.Parallel()

	srv, err := NewWithOptions(Options{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	left := filepath.Join(dir, "left.txt")
	if err := os.WriteFile(left, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.txt")

	body, _ := json.Marshal(map[string]any{"old": left, "new": missing, "mode": "text"})
	request := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(string(body)))
	request.Header.Set(tokenHeader, srv.Token())
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	var answer errorBody
	if err := json.NewDecoder(recorder.Body).Decode(&answer); err != nil {
		t.Fatal(err)
	}
	if answer.Code != CodeFileNotFound {
		t.Errorf("code = %q, want %q", answer.Code, CodeFileNotFound)
	}
	if answer.Path != missing {
		t.Errorf("path = %q, want %q", answer.Path, missing)
	}
	if answer.Side != "right" {
		t.Errorf("side = %q, want right", answer.Side)
	}
	if !strings.Contains(answer.Error, "right:") {
		t.Errorf("the API message changed shape: %q", answer.Error)
	}
}

func TestMalformedJSONAnswersWithInvalidRequest(t *testing.T) {
	t.Parallel()

	srv, err := NewWithOptions(Options{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader("{not json"))
	request.Header.Set(tokenHeader, srv.Token())
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var answer errorBody
	if err := json.NewDecoder(recorder.Body).Decode(&answer); err != nil {
		t.Fatal(err)
	}
	if answer.Code != CodeInvalidRequest {
		t.Errorf("code = %q, want %q", answer.Code, CodeInvalidRequest)
	}
}

func TestAnUnauthenticatedCallIsCoded(t *testing.T) {
	t.Parallel()

	srv, err := NewWithOptions(Options{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader("{}"))
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var answer errorBody
	if err := json.NewDecoder(recorder.Body).Decode(&answer); err != nil {
		t.Fatal(err)
	}
	if answer.Code != CodeUnauthorized {
		t.Errorf("code = %q, want %q", answer.Code, CodeUnauthorized)
	}
}
