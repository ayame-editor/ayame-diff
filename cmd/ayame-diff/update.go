package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hjosugi/ayame-diff/internal/selfupdate"
)

// runUpdate implements: ayame-diff update [--check]
func runUpdate(args []string) {
	fs := flag.NewFlagSet("ayame-diff update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
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
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if checkOnly {
		rel, err := selfupdate.LatestRelease(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		if selfupdate.NeedsUpdate(version, rel.TagName) {
			fmt.Printf("update available: %s -> %s\n%s\n", version, rel.TagName, rel.HTMLURL)
		} else {
			fmt.Printf("up to date (%s)\n", version)
		}
		return
	}

	if err := selfupdate.Update(ctx, version, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runRemove implements: ayame-diff remove [--yes]
func runRemove(args []string) {
	fs := flag.NewFlagSet("ayame-diff remove", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
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
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	if mgr := selfupdate.ManagedInstall(); mgr != "" {
		fmt.Fprintf(os.Stderr, "this install is managed by %s; use %s to remove it\n", mgr, strings.ToLower(mgr))
		os.Exit(1)
	}
	if !yes {
		exe, _ := os.Executable()
		fmt.Fprintf(os.Stderr, "remove %s? [y/N] ", exe)
		if !confirm() {
			fmt.Fprintln(os.Stderr, "cancelled")
			return
		}
	}
	if err := selfupdate.Remove(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func confirm() bool {
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(sc.Text()))
	return ans == "y" || ans == "yes"
}
