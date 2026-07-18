package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hjosugi/ayame-diff/internal/selfupdate"
)

func TestConfirm(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		input string
		want  bool
	}{
		{input: "y\n", want: true},
		{input: " YES \n", want: true},
		{input: "n\n", want: false},
		{input: "\n", want: false},
		{input: "", want: false},
	} {
		if got := confirm(strings.NewReader(tt.input)); got != tt.want {
			t.Errorf("confirm(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRunUpdateCheck(t *testing.T) {
	t.Parallel()
	rel := &selfupdate.Release{TagName: "v9.9.9", HTMLURL: "https://example.test/release"}
	deps := updateCommandDeps{
		latestRelease: func(context.Context) (*selfupdate.Release, error) { return rel, nil },
		needsUpdate: func(current, latest string) bool {
			if current != version || latest != rel.TagName {
				t.Fatalf("NeedsUpdate(%q, %q)", current, latest)
			}
			return true
		},
		update: func(context.Context, string, io.Writer) error {
			t.Fatal("update called for --check")
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	if code := runUpdateWithDeps([]string{"--check"}, &stdout, &stderr, deps); code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "update available: "+version+" -> "+rel.TagName) || !strings.Contains(got, rel.HTMLURL) {
		t.Fatalf("stdout = %q", got)
	}

	deps.needsUpdate = func(string, string) bool { return false }
	stdout.Reset()
	if code := runUpdateWithDeps([]string{"--check"}, &stdout, &stderr, deps); code != exitOK {
		t.Fatalf("up-to-date code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "up to date ("+version+")") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunUpdateMapsFailuresAndCancellation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "failure", err: errors.New("network unavailable"), code: exitError},
		{name: "canceled", err: context.Canceled, code: exitInterrupt},
	}
	for _, tt := range tests {
		t.Run("check/"+tt.name, func(t *testing.T) {
			deps := updateCommandDeps{
				latestRelease: func(context.Context) (*selfupdate.Release, error) { return nil, tt.err },
				needsUpdate:   selfupdate.NeedsUpdate,
				update:        selfupdate.Update,
			}
			var stdout, stderr bytes.Buffer
			if code := runUpdateWithDeps([]string{"--check"}, &stdout, &stderr, deps); code != tt.code {
				t.Fatalf("code = %d, want %d", code, tt.code)
			}
			if !strings.Contains(stderr.String(), tt.err.Error()) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
		t.Run("install/"+tt.name, func(t *testing.T) {
			deps := updateCommandDeps{
				latestRelease: selfupdate.LatestRelease,
				needsUpdate:   selfupdate.NeedsUpdate,
				update: func(_ context.Context, current string, _ io.Writer) error {
					if current != version {
						t.Fatalf("current version = %q, want %q", current, version)
					}
					return tt.err
				},
			}
			var stdout, stderr bytes.Buffer
			if code := runUpdateWithDeps(nil, &stdout, &stderr, deps); code != tt.code {
				t.Fatalf("code = %d, want %d", code, tt.code)
			}
			if !strings.Contains(stderr.String(), tt.err.Error()) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunUpdateInstallSuccessAndFlagError(t *testing.T) {
	t.Parallel()
	called := false
	deps := updateCommandDeps{
		latestRelease: selfupdate.LatestRelease,
		needsUpdate:   selfupdate.NeedsUpdate,
		update: func(_ context.Context, _ string, w io.Writer) error {
			called = true
			_, _ = io.WriteString(w, "installed safely\n")
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	if code := runUpdateWithDeps(nil, &stdout, &stderr, deps); code != exitOK || !called {
		t.Fatalf("code = %d, called = %v, stderr=%q", code, called, stderr.String())
	}
	if stdout.String() != "installed safely\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runUpdateWithDeps([]string{"--unknown"}, &stdout, &stderr, deps); code != exitUsage {
		t.Fatalf("flag error code = %d, want %d", code, exitUsage)
	}
}

func TestRunRemoveDestructiveBranches(t *testing.T) {
	t.Parallel()
	newDeps := func(input string) (removeCommandDeps, *bool) {
		removed := new(bool)
		return removeCommandDeps{
			managedInstall: func() string { return "" },
			executable:     func() (string, error) { return "/tmp/standalone-ayame-diff", nil },
			remove: func(w io.Writer) error {
				*removed = true
				_, _ = io.WriteString(w, "removed test binary\n")
				return nil
			},
			stdin: strings.NewReader(input),
		}, removed
	}

	t.Run("managed install is refused", func(t *testing.T) {
		deps, removed := newDeps("yes\n")
		deps.managedInstall = func() string { return "Homebrew" }
		var stdout, stderr bytes.Buffer
		if code := runRemoveWithDeps(nil, &stdout, &stderr, deps); code != exitError {
			t.Fatalf("code = %d, want %d", code, exitError)
		}
		if *removed || !strings.Contains(stderr.String(), "managed by Homebrew") {
			t.Fatalf("removed=%v stderr=%q", *removed, stderr.String())
		}
	})

	t.Run("confirmation declined", func(t *testing.T) {
		deps, removed := newDeps("no\n")
		var stdout, stderr bytes.Buffer
		if code := runRemoveWithDeps(nil, &stdout, &stderr, deps); code != exitInterrupt {
			t.Fatalf("code = %d, want %d", code, exitInterrupt)
		}
		if *removed || !strings.Contains(stderr.String(), "remove /tmp/standalone-ayame-diff?") || !strings.Contains(stderr.String(), "cancelled") {
			t.Fatalf("removed=%v stderr=%q", *removed, stderr.String())
		}
	})

	t.Run("confirmation accepted", func(t *testing.T) {
		deps, removed := newDeps("yes\n")
		var stdout, stderr bytes.Buffer
		if code := runRemoveWithDeps(nil, &stdout, &stderr, deps); code != exitOK {
			t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
		}
		if !*removed || !strings.Contains(stdout.String(), "removed test binary") {
			t.Fatalf("removed=%v stdout=%q", *removed, stdout.String())
		}
	})

	t.Run("yes flag bypasses prompt", func(t *testing.T) {
		deps, removed := newDeps("")
		deps.executable = func() (string, error) {
			t.Fatal("executable queried with --yes")
			return "", nil
		}
		var stdout, stderr bytes.Buffer
		if code := runRemoveWithDeps([]string{"--yes"}, &stdout, &stderr, deps); code != exitOK || !*removed {
			t.Fatalf("code = %d, removed=%v, stderr=%q", code, *removed, stderr.String())
		}
	})

	t.Run("remove failure", func(t *testing.T) {
		deps, _ := newDeps("")
		deps.remove = func(io.Writer) error { return errors.New("permission denied") }
		var stdout, stderr bytes.Buffer
		if code := runRemoveWithDeps([]string{"--yes"}, &stdout, &stderr, deps); code != exitError {
			t.Fatalf("code = %d, want %d", code, exitError)
		}
		if !strings.Contains(stderr.String(), "permission denied") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})

	t.Run("invalid flag", func(t *testing.T) {
		deps, removed := newDeps("")
		var stdout, stderr bytes.Buffer
		if code := runRemoveWithDeps([]string{"--unknown"}, &stdout, &stderr, deps); code != exitUsage {
			t.Fatalf("code = %d, want %d", code, exitUsage)
		}
		if *removed {
			t.Fatal("remove called after flag error")
		}
	})
}
