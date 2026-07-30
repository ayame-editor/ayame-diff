package main

import (
	"context"
	"io"
	"net"
	"net/http"
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
