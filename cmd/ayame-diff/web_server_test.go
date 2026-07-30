package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/hjosugi/ayame-diff/internal/server"
)

func TestServeUntilContextGracefullyStops(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() {
		stopped <- serveUntilContext(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}), nil)
	}()

	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	_ = response.Body.Close()
	cancel()

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("serveUntilContext: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not shut down after context cancellation")
	}
}

func TestListenWithPortFallbackUsesNextAvailablePort(t *testing.T) {
	t.Parallel()

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port
	if port == 65535 {
		t.Skip("ephemeral listener selected the last TCP port")
	}

	requested := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, fellBack, err := listenWithPortFallback(net.Listen, "tcp", requested)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if !fellBack {
		t.Fatal("listen did not report a port fallback")
	}
	if listener.Addr().String() == requested {
		t.Fatalf("fallback listener still uses occupied address %s", requested)
	}
}

func TestListenWithPortFallbackPreservesOtherErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("listen failed")
	calls := 0
	listener, fellBack, err := listenWithPortFallback(
		func(string, string) (net.Listener, error) {
			calls++
			return nil, want
		},
		"tcp",
		"127.0.0.1:8080",
	)
	if listener != nil || fellBack || !errors.Is(err, want) {
		t.Fatalf("listener=%v fellBack=%v err=%v", listener, fellBack, err)
	}
	if calls != 1 {
		t.Fatalf("listen calls = %d, want 1", calls)
	}
}

func TestListenWithPortFallbackStopsOnNonConflictCandidateError(t *testing.T) {
	t.Parallel()

	calls := 0
	listener, fellBack, err := listenWithPortFallback(
		func(string, string) (net.Listener, error) {
			calls++
			if calls == 1 {
				return nil, syscall.EADDRINUSE
			}
			return nil, syscall.EACCES
		},
		"tcp",
		"127.0.0.1:8080",
	)
	if listener != nil || fellBack || !errors.Is(err, syscall.EACCES) {
		t.Fatalf("listener=%v fellBack=%v err=%v", listener, fellBack, err)
	}
	if calls != 2 {
		t.Fatalf("listen calls = %d, want 2", calls)
	}
}

func TestShutdownEndpointStopsOwnedServer(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	shutdownRequests := make(chan struct{}, 1)
	handler, token, err := newServerHandler("test", listener.Addr(), false, server.LifecycleOptions{
		Shutdown: func() { shutdownRequests <- struct{}{} },
	})
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() {
		stopped <- serveUntilContext(context.Background(), listener, handler, shutdownRequests)
	}()

	request, err := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+"/api/shutdown", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Ayame-Token", token)
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("shutdown status = %d", response.StatusCode)
	}

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("serveUntilContext: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not shut down after authenticated API request")
	}
}
