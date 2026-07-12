package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hjosugi/ayame-diff/internal/shellintegration"
)

func runShellInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff shell-install", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "ayame-diff shell-install\n\nRegister current-user Explorer, Finder, or Linux file-manager integration.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: shell-install takes no arguments")
		return 2
	}
	env, err := shellEnvironment()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	paths, err := shellintegration.Install(env)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, "file-manager integration installed")
	for _, path := range paths {
		fmt.Fprintln(stdout, path)
	}
	return 0
}

func runShellUninstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ayame-diff shell-uninstall", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "ayame-diff shell-uninstall\n\nRemove current-user file-manager integration.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: shell-uninstall takes no arguments")
		return 2
	}
	env, err := shellEnvironment()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if err := shellintegration.Uninstall(env); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, "file-manager integration removed")
	return 0
}

func shellEnvironment() (shellintegration.Environment, error) {
	executable, err := os.Executable()
	if err != nil {
		return shellintegration.Environment{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return shellintegration.Environment{}, err
	}
	return shellintegration.Environment{GOOS: runtime.GOOS, Executable: executable, Home: home, AppData: os.Getenv("APPDATA")}, nil
}

type shellSelection struct {
	Path string    `json:"path"`
	Time time.Time `json:"time"`
}

// runShellSelect implements WinMerge-style two-step Explorer selection. The
// first invocation records a path; the second clears it and launches the GUI.
func runShellSelect(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] == "" {
		fmt.Fprintln(stderr, "error: shell-select needs one path")
		return 2
	}
	config, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	statePath := filepath.Join(config, "ayame-diff", "shell-selection.json")
	data, readErr := os.ReadFile(statePath)
	var previous shellSelection
	valid := readErr == nil && json.Unmarshal(data, &previous) == nil && previous.Path != "" && time.Since(previous.Time) < 30*time.Minute
	if valid && previous.Path != args[0] {
		_ = os.Remove(statePath)
		return runGUI([]string{previous.Path, args[0]}, stdout, stderr)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	data, _ = json.Marshal(shellSelection{Path: args[0], Time: time.Now()})
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, "first path selected; choose the second path with Compare with Ayame Diff")
	return 0
}
