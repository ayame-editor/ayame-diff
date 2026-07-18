package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"

	"github.com/hjosugi/ayame-diff/internal/server"
)

// runGUI implements: ayame-diff gui [--addr host:port] [--allow-remote]
// [--no-open] [OLD [NEW]]
//
// It starts the same local web UI as `serve` but, by default, binds an
// ephemeral localhost port and opens the browser — the "double-click to a GUI"
// experience without a native webview dependency (keeping the single static,
// cross-compiled binary). See ADR 0002 / hjosugi/ayame-diff#14.
func runGUI(args []string, stdout, stderr io.Writer) int {
	return runGUIWithDeps(args, stdout, stderr, guiCommandDeps{
		newHandler: newServerHandler,
		listen:     net.Listen,
		serve: func(ln net.Listener, handler http.Handler) error {
			return server.NewHTTPServer("", handler).Serve(ln)
		},
		openBrowser: openBrowser,
	})
}

type guiCommandDeps struct {
	newHandler  func(string) (http.Handler, error)
	listen      func(string, string) (net.Listener, error)
	serve       func(net.Listener, http.Handler) error
	openBrowser func(string) error
}

func runGUIWithDeps(args []string, stdout, stderr io.Writer, deps guiCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff gui", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var addr string
	var noOpen bool
	var allowRemote bool
	fs.StringVar(&addr, "addr", "127.0.0.1:0", "listen address; port 0 picks a free port")
	fs.BoolVar(&noOpen, "no-open", false, "start the server but do not open the browser")
	fs.BoolVar(&allowRemote, "allow-remote", false, "allow a non-loopback listen address (unsafe without network access controls)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff gui [--addr host:port] [--allow-remote] [--no-open] [OLD [NEW]]

Start the local web UI and open it in your browser. Same UI as `+"`serve`"+`, but
picks a free localhost port and launches the browser for you. With two paths,
the GUI chooses text/folder mode and starts comparing immediately. Non-loopback
addresses require the explicit --allow-remote safety opt-in.`)
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
	if fs.NArg() > 2 {
		fmt.Fprintln(stderr, "error: gui accepts at most two paths: OLD NEW")
		return exitUsage
	}
	remote, err := remoteBind(addr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	if remote && !allowRemote {
		fmt.Fprintln(stderr, "error: non-loopback listen addresses require --allow-remote")
		return exitUsage
	}

	handler, err := deps.newHandler(version)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	ln, err := deps.listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer ln.Close()
	if remote {
		printRemoteWarning(stderr)
	}
	guiURL := guiLaunchURL(browserBaseURL(ln.Addr()), fs.Args())
	fmt.Fprintf(stderr, "ayame-diff GUI at %s  (Ctrl+C to stop)\n", guiURL)
	if !noOpen {
		if err := deps.openBrowser(guiURL); err != nil {
			fmt.Fprintf(stderr, "could not open a browser automatically (%v); open %s manually\n", err, guiURL)
		}
	}
	if err := deps.serve(ln, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	return exitOK
}

func guiLaunchURL(base string, paths []string) string {
	if len(paths) == 0 {
		return base
	}
	query := url.Values{"old": {paths[0]}}
	if len(paths) == 2 {
		query.Set("new", paths[1])
		query.Set("autorun", "1")
		if oldInfo, oldErr := os.Stat(paths[0]); oldErr == nil && oldInfo.IsDir() {
			if newInfo, newErr := os.Stat(paths[1]); newErr == nil && newInfo.IsDir() {
				query.Set("mode", "dir")
			}
		}
	}
	return base + "?" + query.Encode()
}

// openBrowser opens url in the platform's default browser. The launcher is
// detached; a failure to start is reported to the caller.
func openBrowser(url string) error {
	name, args := browserCommand(runtime.GOOS, url)
	return exec.Command(name, args...).Start()
}

func browserCommand(goos, url string) (string, []string) {
	var name string
	var args []string
	switch goos {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	return name, args
}
