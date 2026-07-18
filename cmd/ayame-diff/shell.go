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
	return runShellInstallWithDeps(args, stdout, stderr, shellCommandDeps{
		environment: shellEnvironment,
		install:     shellintegration.Install,
	})
}

type shellCommandDeps struct {
	environment func() (shellintegration.Environment, error)
	install     func(shellintegration.Environment) ([]string, error)
	uninstall   func(shellintegration.Environment) error
	configDir   func() (string, error)
	runGUI      func([]string, io.Writer, io.Writer) int
	now         func() time.Time
}

func runShellInstallWithDeps(args []string, stdout, stderr io.Writer, deps shellCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff shell-install", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "ayame-diff shell-install\n\nRegister current-user Explorer, Finder, or Linux file-manager integration.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: shell-install takes no arguments")
		return exitUsage
	}
	env, err := deps.environment()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	paths, err := deps.install(env)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	fmt.Fprintln(stdout, "file-manager integration installed")
	for _, path := range paths {
		fmt.Fprintln(stdout, path)
	}
	return exitOK
}

func runShellUninstall(args []string, stdout, stderr io.Writer) int {
	return runShellUninstallWithDeps(args, stdout, stderr, shellCommandDeps{
		environment: shellEnvironment,
		uninstall:   shellintegration.Uninstall,
	})
}

func runShellUninstallWithDeps(args []string, stdout, stderr io.Writer, deps shellCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff shell-uninstall", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "ayame-diff shell-uninstall\n\nRemove current-user file-manager integration.")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: shell-uninstall takes no arguments")
		return exitUsage
	}
	env, err := deps.environment()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	if err := deps.uninstall(env); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	fmt.Fprintln(stdout, "file-manager integration removed")
	return exitOK
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
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, `ayame-diff shell-select PATH

Handle one Windows Explorer "Compare with Ayame Diff" selection. The first
invocation remembers PATH; a second invocation opens both paths in the GUI.
This integration helper is normally invoked by Explorer, not by hand.`)
		return exitOK
	}
	return runShellSelectWithDeps(args, stdout, stderr, shellCommandDeps{
		configDir: os.UserConfigDir,
		runGUI:    runGUI,
		now:       time.Now,
	})
}

func runShellSelectWithDeps(args []string, stdout, stderr io.Writer, deps shellCommandDeps) int {
	if len(args) != 1 || args[0] == "" {
		fmt.Fprintln(stderr, "error: shell-select needs one path")
		return exitUsage
	}
	config, err := deps.configDir()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	statePath := filepath.Join(config, "ayame-diff", "shell-selection.json")
	data, readErr := os.ReadFile(statePath)
	var previous shellSelection
	valid := readErr == nil && json.Unmarshal(data, &previous) == nil && previous.Path != "" && deps.now().Sub(previous.Time) < 30*time.Minute
	if valid && previous.Path != args[0] {
		_ = os.Remove(statePath)
		return deps.runGUI([]string{previous.Path, args[0]}, stdout, stderr)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	data, _ = json.Marshal(shellSelection{Path: args[0], Time: deps.now()})
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	fmt.Fprintln(stdout, "first path selected; choose the second path with Compare with Ayame Diff")
	return exitOK
}
