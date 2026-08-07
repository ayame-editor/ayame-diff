package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ayame-editor/ayame-diff/internal/encoding"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
)

type clipboardCommand struct {
	name string
	args []string
}

var loadClipboardBytes = clipboardBytes

func isClipboardPath(path string) bool {
	return path == "clip:" || path == "clipboard:"
}

func readClipboard(encHint string) (linediff.Lines, error) {
	data, err := loadClipboardBytes()
	if err != nil {
		return nil, err
	}
	name := encoding.Detect(data, encHint)
	decoded, err := io.ReadAll(encoding.Decoder(bytes.NewReader(data), name))
	if err != nil {
		return nil, fmt.Errorf("decoding clipboard: %w", err)
	}
	decoded = bytes.TrimPrefix(decoded, []byte("\xef\xbb\xbf"))
	return linediff.SplitTextLines(string(decoded)), nil
}

func clipboardBytes() ([]byte, error) {
	return clipboardBytesWith(runtime.GOOS, os.Getenv, exec.LookPath, func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	})
}

func clipboardBytesWith(
	goos string,
	getenv func(string) string,
	lookPath func(string) (string, error),
	output func(string, ...string) ([]byte, error),
) ([]byte, error) {
	commands := clipboardCommands(goos, getenv)
	if len(commands) == 0 {
		return nil, fmt.Errorf("clipboard input is not supported on %s", goos)
	}
	var failures []string
	for _, candidate := range commands {
		resolved, err := lookPath(candidate.name)
		if err != nil {
			failures = append(failures, candidate.name+": not installed")
			continue
		}
		data, err := output(resolved, candidate.args...)
		if err == nil {
			return data, nil
		}
		failures = append(failures, candidate.name+": "+err.Error())
	}
	return nil, fmt.Errorf("cannot read the OS clipboard; tried %s", strings.Join(failures, "; "))
}

func clipboardCommands(goos string, getenv func(string) string) []clipboardCommand {
	switch goos {
	case "darwin":
		return []clipboardCommand{{name: "pbpaste"}}
	case "windows":
		args := []string{"-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw"}
		return []clipboardCommand{
			{name: "pwsh.exe", args: args},
			{name: "powershell.exe", args: args},
		}
	case "linux", "freebsd", "openbsd", "netbsd":
		wayland := clipboardCommand{name: "wl-paste", args: []string{"--no-newline"}}
		x11 := clipboardCommand{name: "xclip", args: []string{"-selection", "clipboard", "-o"}}
		if getenv("WAYLAND_DISPLAY") != "" {
			return []clipboardCommand{wayland, x11}
		}
		return []clipboardCommand{x11, wayland}
	default:
		return nil
	}
}
