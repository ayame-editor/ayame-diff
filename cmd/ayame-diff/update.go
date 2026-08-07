package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ayame-editor/ayame-diff/internal/selfupdate"
)

// runUpdate implements: ayame-diff update [--check]
func runUpdate(args []string, stdout, stderr io.Writer) int {
	return runUpdateWithDeps(args, stdout, stderr, updateCommandDeps{
		latestRelease: selfupdate.LatestRelease,
		needsUpdate:   selfupdate.NeedsUpdate,
		update:        selfupdate.Update,
	})
}

type updateCommandDeps struct {
	latestRelease func(context.Context) (*selfupdate.Release, error)
	needsUpdate   func(string, string) bool
	update        func(context.Context, string, io.Writer) error
}

func runUpdateWithDeps(args []string, stdout, stderr io.Writer, deps updateCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff update", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var checkOnly bool
	fs.BoolVar(&checkOnly, "check", false, "only report whether a newer release exists; do not install")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff update [--check]

Download and install the latest release from GitHub, replacing this binary.
Verifies the release's SHA-256 checksum before installing.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if checkOnly {
		rel, err := deps.latestRelease(ctx)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			if errors.Is(err, context.Canceled) {
				return exitInterrupt
			}
			return exitError
		}
		if deps.needsUpdate(version, rel.TagName) {
			fmt.Fprintf(stdout, "update available: %s -> %s\n%s\n", version, rel.TagName, rel.HTMLURL)
		} else {
			fmt.Fprintf(stdout, "up to date (%s)\n", version)
		}
		return exitOK
	}

	if err := deps.update(ctx, version, stdout); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		if errors.Is(err, context.Canceled) {
			return exitInterrupt
		}
		return exitError
	}
	return exitOK
}

// runRemove implements: ayame-diff remove [--yes]
func runRemove(args []string, stdout, stderr io.Writer) int {
	return runRemoveWithDeps(args, stdout, stderr, removeCommandDeps{
		managedInstall: selfupdate.ManagedInstall,
		executable:     os.Executable,
		remove:         selfupdate.Remove,
		stdin:          os.Stdin,
	})
}

type removeCommandDeps struct {
	managedInstall func() string
	executable     func() (string, error)
	remove         func(io.Writer) error
	stdin          io.Reader
}

func runRemoveWithDeps(args []string, stdout, stderr io.Writer, deps removeCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff remove", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var yes bool
	fs.BoolVar(&yes, "yes", false, "remove without asking for confirmation")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff remove [--yes]

Uninstall this standalone binary. Managed installs (Homebrew/Scoop/Nix) are
detected and left to their package manager.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	if mgr := deps.managedInstall(); mgr != "" {
		fmt.Fprintf(stderr, "this install is managed by %s; use %s to remove it\n", mgr, strings.ToLower(mgr))
		return exitError
	}
	if !yes {
		exe, _ := deps.executable()
		fmt.Fprintf(stderr, "remove %s? [y/N] ", exe)
		if !confirm(deps.stdin) {
			fmt.Fprintln(stderr, "cancelled")
			return exitInterrupt
		}
	}
	if err := deps.remove(stdout); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	return exitOK
}

func confirm(r io.Reader) bool {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(sc.Text()))
	return ans == "y" || ans == "yes"
}
