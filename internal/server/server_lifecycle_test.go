package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newLifecycleTestServer(t *testing.T, leaseTimeout, closeGrace time.Duration, shutdown func()) (*Server, http.Handler) {
	t.Helper()
	server, err := NewWithOptions(Options{
		Version: "test",
		Lifecycle: LifecycleOptions{
			Shutdown:            shutdown,
			BrowserLeaseTimeout: leaseTimeout,
			BrowserCloseGrace:   closeGrace,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return server, authorizedHandler(server)
}

func postLifecycle(t *testing.T, handler http.Handler, target, session string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	body := []byte(`{"session":"` + session + `"}`)
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body)))
	return recorder
}

func TestShutdownEndpointRequestsShutdownOnce(t *testing.T) {
	requested := make(chan struct{}, 2)
	_, handler := newLifecycleTestServer(t, 0, 0, func() { requested <- struct{}{} })

	for range 2 {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/shutdown", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
		}
	}
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("shutdown was not requested")
	}
	select {
	case <-requested:
		t.Fatal("shutdown was requested more than once")
	default:
	}
}

func TestBrowserLifecycleStopsAfterLastSessionRelease(t *testing.T) {
	requested := make(chan struct{}, 1)
	_, handler := newLifecycleTestServer(t, time.Second, 20*time.Millisecond, func() { requested <- struct{}{} })

	if recorder := postLifecycle(t, handler, "/api/lifecycle/heartbeat", "tab-a"); recorder.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body=%s", recorder.Code, recorder.Body)
	}
	if recorder := postLifecycle(t, handler, "/api/lifecycle/heartbeat", "tab-b"); recorder.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body=%s", recorder.Code, recorder.Body)
	}
	if recorder := postLifecycle(t, handler, "/api/lifecycle/release", "tab-a"); recorder.Code != http.StatusOK {
		t.Fatalf("release status = %d body=%s", recorder.Code, recorder.Body)
	}
	select {
	case <-requested:
		t.Fatal("releasing one of two sessions stopped the server")
	case <-time.After(50 * time.Millisecond):
	}
	if recorder := postLifecycle(t, handler, "/api/lifecycle/release", "tab-b"); recorder.Code != http.StatusOK {
		t.Fatalf("release status = %d body=%s", recorder.Code, recorder.Body)
	}
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("last session release did not stop the server")
	}
}

func TestBrowserLifecycleExpiresWithoutHeartbeat(t *testing.T) {
	requested := make(chan struct{}, 1)
	newLifecycleTestServer(t, 20*time.Millisecond, 10*time.Millisecond, func() { requested <- struct{}{} })

	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("expired browser lease did not stop the server")
	}
}

func TestBrowserLifecycleRejectsUnsafeSession(t *testing.T) {
	_, handler := newLifecycleTestServer(t, time.Second, time.Second, func() {})
	recorder := postLifecycle(t, handler, "/api/lifecycle/heartbeat", "../tab")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body)
	}
}
