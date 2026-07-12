package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestClipboardBackendSelection(t *testing.T) {
	t.Parallel()
	emptyEnv := func(string) string { return "" }
	waylandEnv := func(key string) string {
		if key == "WAYLAND_DISPLAY" {
			return "wayland-0"
		}
		return ""
	}
	tests := []struct {
		goos string
		env  func(string) string
		want []string
	}{
		{goos: "darwin", env: emptyEnv, want: []string{"pbpaste"}},
		{goos: "windows", env: emptyEnv, want: []string{"pwsh.exe", "powershell.exe"}},
		{goos: "linux", env: emptyEnv, want: []string{"xclip", "wl-paste"}},
		{goos: "linux", env: waylandEnv, want: []string{"wl-paste", "xclip"}},
	}
	for _, tt := range tests {
		commands := clipboardCommands(tt.goos, tt.env)
		got := make([]string, len(commands))
		for i, command := range commands {
			got[i] = command.name
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s commands=%v, want %v", tt.goos, got, tt.want)
		}
	}
}

func TestClipboardFallsBackToAvailableBackend(t *testing.T) {
	t.Parallel()
	lookPath := func(name string) (string, error) {
		if name == "xclip" {
			return "/usr/bin/xclip", nil
		}
		return "", errors.New("missing")
	}
	var command string
	data, err := clipboardBytesWith("linux", func(string) string { return "wayland-0" }, lookPath, func(name string, args ...string) ([]byte, error) {
		command = name + " " + strings.Join(args, " ")
		return []byte("clipboard text\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "clipboard text\n" || command != "/usr/bin/xclip -selection clipboard -o" {
		t.Fatalf("data=%q command=%q", data, command)
	}
}

func TestClipboardReportsMissingBackends(t *testing.T) {
	t.Parallel()
	_, err := clipboardBytesWith(
		"linux",
		func(string) string { return "" },
		func(string) (string, error) { return "", errors.New("missing") },
		func(string, ...string) ([]byte, error) { return nil, nil },
	)
	if err == nil || !strings.Contains(err.Error(), "xclip: not installed") || !strings.Contains(err.Error(), "wl-paste: not installed") {
		t.Fatalf("err=%v", err)
	}
}

func TestClipboardPathAliases(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"clip:", "clipboard:"} {
		if !isClipboardPath(path) {
			t.Errorf("%q should be a clipboard path", path)
		}
	}
	if isClipboardPath("clip.txt") {
		t.Fatal("ordinary path was treated as clipboard")
	}
}

func TestRunTextReadsClipboardPseudoPath(t *testing.T) {
	original := loadClipboardBytes
	loadClipboardBytes = func() ([]byte, error) { return []byte("from clipboard\n"), nil }
	defer func() { loadClipboardBytes = original }()

	file := filepath.Join(t.TempDir(), "saved.txt")
	if err := os.WriteFile(file, []byte("from file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runText([]string{"clip:", file}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "-from clipboard") || !strings.Contains(stdout.String(), "+from file") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
