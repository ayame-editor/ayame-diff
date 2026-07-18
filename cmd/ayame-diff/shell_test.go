package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hjosugi/ayame-diff/internal/shellintegration"
)

func TestRunShellInstallAndUninstallUseIsolatedHome(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	env := shellintegration.Environment{
		GOOS:       "linux",
		Executable: "/opt/Ayame Diff/ayame-diff",
		Home:       home,
	}
	deps := shellCommandDeps{
		environment: func() (shellintegration.Environment, error) { return env, nil },
		install:     shellintegration.Install,
		uninstall:   shellintegration.Uninstall,
	}

	for i := 0; i < 2; i++ {
		var stdout, stderr bytes.Buffer
		if code := runShellInstallWithDeps(nil, &stdout, &stderr, deps); code != exitOK {
			t.Fatalf("install %d code = %d, want %d; stderr=%q", i, code, exitOK, stderr.String())
		}
		if !strings.Contains(stdout.String(), "file-manager integration installed") {
			t.Fatalf("install stdout = %q", stdout.String())
		}
	}
	desktop := filepath.Join(home, ".local", "share", "applications", "ayame-diff.desktop")
	data, err := os.ReadFile(desktop)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `Exec="/opt/Ayame Diff/ayame-diff" --gui %F`) {
		t.Fatalf("desktop entry = %q", data)
	}

	for i := 0; i < 2; i++ {
		var stdout, stderr bytes.Buffer
		if code := runShellUninstallWithDeps(nil, &stdout, &stderr, deps); code != exitOK {
			t.Fatalf("uninstall %d code = %d, want %d; stderr=%q", i, code, exitOK, stderr.String())
		}
		if !strings.Contains(stdout.String(), "file-manager integration removed") {
			t.Fatalf("uninstall stdout = %q", stdout.String())
		}
	}
	if _, err := os.Stat(desktop); !os.IsNotExist(err) {
		t.Fatalf("desktop entry still exists: %v", err)
	}
}

func TestRunShellInstallAndUninstallErrors(t *testing.T) {
	t.Parallel()
	envErr := errors.New("no home")
	opErr := errors.New("operation failed")
	tests := []struct {
		name string
		run  func(*bytes.Buffer, *bytes.Buffer) int
		want string
	}{
		{
			name: "install argument",
			run: func(stdout, stderr *bytes.Buffer) int {
				return runShellInstallWithDeps([]string{"extra"}, stdout, stderr, shellCommandDeps{})
			},
			want: "takes no arguments",
		},
		{
			name: "install environment",
			run: func(stdout, stderr *bytes.Buffer) int {
				return runShellInstallWithDeps(nil, stdout, stderr, shellCommandDeps{
					environment: func() (shellintegration.Environment, error) { return shellintegration.Environment{}, envErr },
				})
			},
			want: envErr.Error(),
		},
		{
			name: "install operation",
			run: func(stdout, stderr *bytes.Buffer) int {
				return runShellInstallWithDeps(nil, stdout, stderr, shellCommandDeps{
					environment: func() (shellintegration.Environment, error) { return shellintegration.Environment{}, nil },
					install:     func(shellintegration.Environment) ([]string, error) { return nil, opErr },
				})
			},
			want: opErr.Error(),
		},
		{
			name: "uninstall argument",
			run: func(stdout, stderr *bytes.Buffer) int {
				return runShellUninstallWithDeps([]string{"extra"}, stdout, stderr, shellCommandDeps{})
			},
			want: "takes no arguments",
		},
		{
			name: "uninstall environment",
			run: func(stdout, stderr *bytes.Buffer) int {
				return runShellUninstallWithDeps(nil, stdout, stderr, shellCommandDeps{
					environment: func() (shellintegration.Environment, error) { return shellintegration.Environment{}, envErr },
				})
			},
			want: envErr.Error(),
		},
		{
			name: "uninstall operation",
			run: func(stdout, stderr *bytes.Buffer) int {
				return runShellUninstallWithDeps(nil, stdout, stderr, shellCommandDeps{
					environment: func() (shellintegration.Environment, error) { return shellintegration.Environment{}, nil },
					uninstall:   func(shellintegration.Environment) error { return opErr },
				})
			},
			want: opErr.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := tt.run(&stdout, &stderr)
			wantCode := exitError
			if strings.Contains(tt.name, "argument") {
				wantCode = exitUsage
			}
			if code != wantCode {
				t.Fatalf("code = %d, want %d", code, wantCode)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestShellEnvironment(t *testing.T) {
	t.Parallel()
	env, err := shellEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if env.GOOS != runtime.GOOS {
		t.Fatalf("GOOS = %q, want %q", env.GOOS, runtime.GOOS)
	}
	if env.Executable == "" || !filepath.IsAbs(env.Executable) {
		t.Fatalf("Executable = %q, want absolute path", env.Executable)
	}
	if env.Home == "" {
		t.Fatal("Home is empty")
	}
}

func TestRunShellSelectUsesUserConfigDirectory(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("APPDATA", config)
	t.Setenv("HOME", config)

	var stdout, stderr bytes.Buffer
	if code := runShellSelect([]string{"/tmp/first"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	statePath := filepath.Join(config, "ayame-diff", "shell-selection.json")
	if runtime.GOOS == "darwin" {
		statePath = filepath.Join(config, "Library", "Application Support", "ayame-diff", "shell-selection.json")
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("selection state: %v", err)
	}
}

func TestRunShellSelectTwoStepFlow(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	var guiArgs []string
	deps := shellCommandDeps{
		configDir: func() (string, error) { return config, nil },
		runGUI: func(args []string, _, _ io.Writer) int {
			guiArgs = append([]string(nil), args...)
			return exitOK
		},
		now: func() time.Time { return now },
	}
	statePath := filepath.Join(config, "ayame-diff", "shell-selection.json")

	var stdout, stderr bytes.Buffer
	if code := runShellSelectWithDeps([]string{"/tmp/old"}, &stdout, &stderr, deps); code != exitOK {
		t.Fatalf("first code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var selected shellSelection
	if err := json.Unmarshal(data, &selected); err != nil {
		t.Fatal(err)
	}
	if selected.Path != "/tmp/old" || !selected.Time.Equal(now) {
		t.Fatalf("selection = %+v", selected)
	}
	if len(guiArgs) != 0 || !strings.Contains(stdout.String(), "first path selected") {
		t.Fatalf("guiArgs=%v stdout=%q", guiArgs, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runShellSelectWithDeps([]string{"/tmp/new"}, &stdout, &stderr, deps); code != exitOK {
		t.Fatalf("second code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	if !reflect.DeepEqual(guiArgs, []string{"/tmp/old", "/tmp/new"}) {
		t.Fatalf("GUI args = %v", guiArgs)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("selection state still exists: %v", err)
	}
}

func TestRunShellSelectRejectsBadInputAndExpiresOldSelection(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	deps := shellCommandDeps{
		configDir: func() (string, error) { return "", errors.New("config unavailable") },
		runGUI: func([]string, io.Writer, io.Writer) int {
			t.Fatal("GUI called")
			return exitError
		},
		now: time.Now,
	}
	if code := runShellSelectWithDeps(nil, &stdout, &stderr, deps); code != exitUsage {
		t.Fatalf("missing path code = %d, want %d", code, exitUsage)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runShellSelectWithDeps([]string{"path"}, &stdout, &stderr, deps); code != exitError {
		t.Fatalf("config error code = %d, want %d", code, exitError)
	}

	config := t.TempDir()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	statePath := filepath.Join(config, "ayame-diff", "shell-selection.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	old, err := json.Marshal(shellSelection{Path: "/tmp/expired", Time: now.Add(-31 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, old, 0o600); err != nil {
		t.Fatal(err)
	}
	deps.configDir = func() (string, error) { return config, nil }
	deps.now = func() time.Time { return now }
	stdout.Reset()
	stderr.Reset()
	if code := runShellSelectWithDeps([]string{"/tmp/new"}, &stdout, &stderr, deps); code != exitOK {
		t.Fatalf("expired code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var selected shellSelection
	if err := json.Unmarshal(data, &selected); err != nil {
		t.Fatal(err)
	}
	if selected.Path != "/tmp/new" {
		t.Fatalf("selection = %+v", selected)
	}
}
