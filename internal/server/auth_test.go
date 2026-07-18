package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRawTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New("test")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestAPIRequiresToken is the #108 regression. Without it, any website the user
// happened to be visiting could POST to the local server and have it read or
// overwrite arbitrary files, and could GET /api/files to walk the disk. The GET
// endpoints matter most: the Origin gate (#145) exempts safe methods, so they
// had no protection at all.
func TestAPIRequiresToken(t *testing.T) {
	t.Parallel()
	s := newRawTestServer(t)
	handler := s.Handler()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/files?path=/", ""},
		{http.MethodGet, "/api/path-info?path=/etc/passwd", ""},
		{http.MethodPost, "/api/diff", `{}`},
		{http.MethodPost, "/api/merge/text", `{}`},
		{http.MethodPost, "/api/csv/export", `{}`},
		{http.MethodPost, "/api/merge/csv", `{}`},
		{http.MethodPost, "/api/merge/three-way/text", `{}`},
		{http.MethodPost, "/api/merge/three-way/csv", `{}`},
		{http.MethodPost, "/api/project/save", `{}`},
		{http.MethodPost, "/api/drop?session=x&name=y", "data"},
		{http.MethodPost, "/api/dir/diff", `{}`},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("code=%d want 401 — this endpoint is reachable without the token", rec.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
				t.Errorf("body=%q must be the standard JSON error shape", rec.Body.String())
			}
		})
	}
}

// TestTokenGrantsAccess proves the credential actually works, so the refusal
// above is authentication and not a route that is simply broken.
func TestTokenGrantsAccess(t *testing.T) {
	t.Parallel()
	s := newRawTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(`{}`))
	req.Header.Set(tokenHeader, s.Token())
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	// An empty body is a 400; the point is that it got past the auth layer.
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("the correct token was rejected: %s", rec.Body.String())
	}
}

// TestWrongTokenIsRejected covers the near-miss case.
func TestWrongTokenIsRejected(t *testing.T) {
	t.Parallel()
	s := newRawTestServer(t)
	for _, token := range []string{"", "not-the-token", s.Token() + "x", s.Token()[:len(s.Token())-1]} {
		req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(`{}`))
		req.Header.Set(tokenHeader, token)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: code=%d want 401", token, rec.Code)
		}
	}
}

// TestUIAndHealthStayOpen keeps the parts that must work without the token
// working: the embedded page cannot set headers for its own sub-resources, and
// the launching command polls health for readiness before a browser exists.
func TestUIAndHealthStayOpen(t *testing.T) {
	t.Parallel()
	handler := newRawTestServer(t).Handler()
	for _, path := range []string{"/", "/app.js", "/style.css", "/api/health"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("%s requires a token but nothing can supply one there", path)
		}
	}
}

// TestEachServerGetsItsOwnToken keeps a token from one run authorizing another.
func TestEachServerGetsItsOwnToken(t *testing.T) {
	t.Parallel()
	first, second := newRawTestServer(t), newRawTestServer(t)
	if first.Token() == second.Token() {
		t.Fatal("two servers share a token")
	}
	if len(first.Token()) < 32 {
		t.Errorf("token %q is too short to resist guessing", first.Token())
	}
	req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(`{}`))
	req.Header.Set(tokenHeader, second.Token())
	rec := httptest.NewRecorder()
	first.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("another server's token was accepted: code=%d", rec.Code)
	}
}

// TestHostAllowlistBlocksDNSRebinding is the other half of #108. A page on any
// site can make its own hostname resolve to 127.0.0.1 and reach this server,
// but the request still carries that site's name in Host — which is exactly
// what the allowlist checks. Note this fires even with a valid token, since a
// rebinding attacker could otherwise phish one.
func TestHostAllowlistBlocksDNSRebinding(t *testing.T) {
	t.Parallel()
	s, err := NewWithOptions(Options{Version: "test", AllowedHosts: []string{"127.0.0.1:8080", "localhost:8080"}})
	if err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	check := func(host string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q: code=%d want %d", host, rec.Code, want)
		}
	}
	check("attacker.example.com:8080", http.StatusForbidden)
	check("127.0.0.1:9999", http.StatusForbidden) // right host, wrong port
	check("127.0.0.1:8080", http.StatusOK)
	check("LOCALHOST:8080", http.StatusOK) // Host is case-insensitive

	// The static UI is refused too: a rebinding page must not even be able to
	// confirm the server is there.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "attacker.example.com:8080"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("static UI served to a foreign Host: code=%d", rec.Code)
	}
}

// TestEmptyHostAllowlistAcceptsAnyHost documents the remote-listener case: the
// names such a server is reachable under cannot be enumerated, so the token is
// the defense there and Host is not pinned.
func TestEmptyHostAllowlistAcceptsAnyHost(t *testing.T) {
	t.Parallel()
	handler := newRawTestServer(t).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "anything.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code=%d want 200", rec.Code)
	}
}
