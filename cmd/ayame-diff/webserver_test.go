package main

import (
	"bytes"
	"net"
	"strings"
	"testing"
)

func TestRemoteBindRequiresExplicitOptInForNonLoopbackHosts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		addr   string
		remote bool
	}{
		{"127.0.0.1:8080", false},
		{"[::1]:8080", false},
		{"localhost:8080", false},
		{"0.0.0.0:8080", true},
		{"[::]:8080", true},
		{":8080", true},
		{"192.0.2.10:8080", true},
		{"example.test:8080", true},
	} {
		got, err := remoteBind(test.addr)
		if err != nil {
			t.Errorf("remoteBind(%q): %v", test.addr, err)
		} else if got != test.remote {
			t.Errorf("remoteBind(%q) = %v, want %v", test.addr, got, test.remote)
		}
	}
	if _, err := remoteBind("missing-port"); err == nil {
		t.Fatal("remoteBind accepted an address without a port")
	}
}

func TestBrowserBaseURLReplacesWildcardWithUsableLoopback(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		addr string
		want string
	}{
		{"0.0.0.0:4321", "http://127.0.0.1:4321/"},
		{"[::]:4321", "http://[::1]:4321/"},
		{"127.0.0.1:4321", "http://127.0.0.1:4321/"},
		{"[::1]:4321", "http://[::1]:4321/"},
	} {
		got := browserBaseURL(staticAddr(test.addr))
		if got != test.want {
			t.Errorf("browserBaseURL(%q) = %q, want %q", test.addr, got, test.want)
		}
	}
}

func TestServeRejectsRemoteBindWithoutAllowRemote(t *testing.T) {
	t.Parallel()
	for _, runCommand := range []struct {
		name string
		run  func([]string, *bytes.Buffer, *bytes.Buffer) int
	}{
		{"serve", func(args []string, stdout, stderr *bytes.Buffer) int {
			return runServe(args, stdout, stderr)
		}},
		{"gui", func(args []string, stdout, stderr *bytes.Buffer) int {
			return runGUI(args, stdout, stderr)
		}},
	} {
		t.Run(runCommand.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runCommand.run([]string{"--addr", "0.0.0.0:0"}, &stdout, &stderr)
			if code != exitUsage {
				t.Fatalf("code=%d, want %d; stderr=%q", code, exitUsage, stderr.String())
			}
			if !strings.Contains(stderr.String(), "--allow-remote") {
				t.Fatalf("stderr=%q, want explicit opt-in guidance", stderr.String())
			}
		})
	}
}

type staticAddr string

func (a staticAddr) Network() string { return "tcp" }
func (a staticAddr) String() string  { return string(a) }

var _ net.Addr = staticAddr("")
