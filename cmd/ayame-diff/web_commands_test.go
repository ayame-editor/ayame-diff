package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func healthCheckingServe(t *testing.T, ln net.Listener, handler http.Handler) error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		errCh <- http.Serve(ln, handler)
	}()

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{},
	}
	defer client.CloseIdleConnections()
	resp, err := client.Get("http://" + ln.Addr().String() + "/api/health")
	if err != nil {
		_ = ln.Close()
		<-errCh
		t.Fatalf("health request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		_ = ln.Close()
		<-errCh
		t.Fatalf("health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err != nil {
		_ = ln.Close()
		<-errCh
		t.Fatal(err)
	}
	if closeErr != nil {
		_ = ln.Close()
		<-errCh
		t.Fatal(closeErr)
	}
	if !bytes.Contains(body, []byte(`"version":"`+version+`"`)) {
		_ = ln.Close()
		<-errCh
		t.Fatalf("health body = %q", body)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("server stop: %v", err)
	}
	return nil
}

func TestRunServeStartsHealthEndpointAndStops(t *testing.T) {
	t.Parallel()
	deps := serveCommandDeps{
		newHandler: newServerHandler,
		listen:     net.Listen,
		serve: func(ln net.Listener, handler http.Handler) error {
			return healthCheckingServe(t, ln, handler)
		},
	}
	var stdout, stderr bytes.Buffer
	if code := runServeWithDeps([]string{"--addr", "127.0.0.1:0"}, &stdout, &stderr, deps); code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "ayame-diff serving on http://127.0.0.1:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunServeMapsStartupAndServeErrors(t *testing.T) {
	t.Parallel()
	errTest := errors.New("test failure")
	tests := []struct {
		name string
		deps serveCommandDeps
	}{
		{
			name: "handler",
			deps: serveCommandDeps{
				newHandler: func(string, net.Addr, bool) (http.Handler, string, error) { return nil, "", errTest },
				listen:     net.Listen,
			},
		},
		{
			name: "listen",
			deps: serveCommandDeps{
				newHandler: func(string, net.Addr, bool) (http.Handler, string, error) { return http.NotFoundHandler(), "", nil },
				listen:     func(string, string) (net.Listener, error) { return nil, errTest },
			},
		},
		{
			name: "serve",
			deps: serveCommandDeps{
				newHandler: func(string, net.Addr, bool) (http.Handler, string, error) { return http.NotFoundHandler(), "", nil },
				listen:     net.Listen,
				serve:      func(net.Listener, http.Handler) error { return errTest },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runServeWithDeps([]string{"--addr", "127.0.0.1:0"}, &stdout, &stderr, tt.deps); code != exitError {
				t.Fatalf("code = %d, want %d", code, exitError)
			}
			if !strings.Contains(stderr.String(), errTest.Error()) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunGUIStartsHealthEndpointOpensExpectedURLAndStops(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath, newPath := filepath.Join(dir, "old file.txt"), filepath.Join(dir, "new file.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	var opened string
	deps := guiCommandDeps{
		newHandler: newServerHandler,
		listen:     net.Listen,
		serve: func(ln net.Listener, handler http.Handler) error {
			return healthCheckingServe(t, ln, handler)
		},
		openBrowser: func(target string) error {
			opened = target
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	args := []string{"--addr", "127.0.0.1:0", oldPath, newPath}
	if code := runGUIWithDeps(args, &stdout, &stderr, deps); code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	parsed, err := url.Parse(opened)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("old") != oldPath || parsed.Query().Get("new") != newPath || parsed.Query().Get("autorun") != "1" {
		t.Fatalf("opened URL = %q", opened)
	}
	if !strings.Contains(stderr.String(), "ayame-diff GUI at "+opened) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunGUINoOpenAndBrowserFailure(t *testing.T) {
	t.Parallel()
	serve := func(ln net.Listener, handler http.Handler) error {
		return healthCheckingServe(t, ln, handler)
	}
	t.Run("no open", func(t *testing.T) {
		deps := guiCommandDeps{
			newHandler: newServerHandler,
			listen:     net.Listen,
			serve:      serve,
			openBrowser: func(string) error {
				t.Fatal("browser opened with --no-open")
				return nil
			},
		}
		var stdout, stderr bytes.Buffer
		if code := runGUIWithDeps([]string{"--no-open"}, &stdout, &stderr, deps); code != exitOK {
			t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
		}
	})
	t.Run("browser failure is non-fatal", func(t *testing.T) {
		deps := guiCommandDeps{
			newHandler: newServerHandler,
			listen:     net.Listen,
			serve:      serve,
			openBrowser: func(string) error {
				return errors.New("launcher unavailable")
			},
		}
		var stdout, stderr bytes.Buffer
		if code := runGUIWithDeps(nil, &stdout, &stderr, deps); code != exitOK {
			t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
		}
		if !strings.Contains(stderr.String(), "could not open a browser automatically (launcher unavailable)") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
}

func TestRunGUIRejectsTooManyPathsAndMapsServeError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runGUIWithDeps([]string{"one", "two", "three"}, &stdout, &stderr, guiCommandDeps{}); code != exitUsage {
		t.Fatalf("too many paths code = %d, want %d", code, exitUsage)
	}

	errServe := errors.New("serve failed")
	deps := guiCommandDeps{
		newHandler:  func(string, net.Addr, bool) (http.Handler, string, error) { return http.NotFoundHandler(), "", nil },
		listen:      net.Listen,
		serve:       func(net.Listener, http.Handler) error { return errServe },
		openBrowser: func(string) error { return nil },
	}
	stdout.Reset()
	stderr.Reset()
	if code := runGUIWithDeps([]string{"--no-open"}, &stdout, &stderr, deps); code != exitError {
		t.Fatalf("serve error code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), errServe.Error()) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunGUIMapsHandlerAndListenErrors(t *testing.T) {
	t.Parallel()
	errTest := errors.New("startup failed")
	tests := []struct {
		name string
		deps guiCommandDeps
	}{
		{
			name: "handler",
			deps: guiCommandDeps{
				newHandler: func(string, net.Addr, bool) (http.Handler, string, error) { return nil, "", errTest },
				listen:     net.Listen,
			},
		},
		{
			name: "listen",
			deps: guiCommandDeps{
				newHandler: func(string, net.Addr, bool) (http.Handler, string, error) { return http.NotFoundHandler(), "", nil },
				listen:     func(string, string) (net.Listener, error) { return nil, errTest },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runGUIWithDeps([]string{"--no-open"}, &stdout, &stderr, tt.deps); code != exitError {
				t.Fatalf("code = %d, want %d", code, exitError)
			}
			if !strings.Contains(stderr.String(), errTest.Error()) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestBrowserCommand(t *testing.T) {
	t.Parallel()
	target := "https://example.test/a?b=c"
	tests := []struct {
		goos string
		name string
		args []string
	}{
		{goos: "darwin", name: "open", args: []string{target}},
		{goos: "windows", name: "rundll32", args: []string{"url.dll,FileProtocolHandler", target}},
		{goos: "linux", name: "xdg-open", args: []string{target}},
	}
	for _, tt := range tests {
		name, args := browserCommand(tt.goos, target)
		if name != tt.name || !reflect.DeepEqual(args, tt.args) {
			t.Errorf("browserCommand(%q) = %q, %q; want %q, %q", tt.goos, name, args, tt.name, tt.args)
		}
	}
}
