package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSecureCapsJSONBodies covers #147: the secure() middleware bounds JSON POST
// bodies with http.MaxBytesReader, while /api/drop (streamed uploads) is exempt.
// Serial (no t.Parallel) because it temporarily shrinks the package-level cap;
// Go runs parallel tests only after the serial phase, so the mutation is safe.
func TestSecureCapsJSONBodies(t *testing.T) {
	saved := maxJSONBodyBytes
	maxJSONBodyBytes = 16
	defer func() { maxJSONBodyBytes = saved }()

	var gotN int
	var gotErr error
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		gotN, gotErr = len(b), err
		w.WriteHeader(http.StatusOK)
	})
	h := secure(stub)
	over := bytes.Repeat([]byte("x"), 64) // past the 16-byte cap

	// A capped JSON endpoint: reading past the limit fails.
	gotN, gotErr = 0, nil
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(over)))
	if gotErr == nil {
		t.Fatalf("/api/diff body was not capped: read %d bytes without error", gotN)
	}

	// /api/drop is exempt: the whole body streams through.
	gotN, gotErr = 0, nil
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/drop", bytes.NewReader(over)))
	if gotErr != nil || gotN != len(over) {
		t.Fatalf("/api/drop should be exempt from the JSON cap: read %d bytes, err=%v", gotN, gotErr)
	}

	// A within-cap body reads cleanly on a JSON endpoint.
	gotN, gotErr = 0, nil
	under := bytes.Repeat([]byte("y"), 8)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/diff", bytes.NewReader(under)))
	if gotErr != nil || gotN != len(under) {
		t.Fatalf("within-cap body should read fully: read %d bytes, err=%v", gotN, gotErr)
	}
}
