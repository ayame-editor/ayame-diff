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

	"github.com/ayame-editor/ayame-diff/internal/server"
)

// runGUI implements: ayame-diff gui [--addr host:port] [--allow-remote]
// [--no-open] [LEFT [RIGHT]]
//
// It starts the same local web UI as `serve` but, by default, binds an
// ephemeral localhost port and opens the browser — the "double-click to a GUI"
// experience without a native webview dependency (keeping the single static,
// cross-compiled binary). See ADR 0002 / hjosugi/ayame-diff#14.
func runGUI(args []string, stdout, stderr io.Writer) int {
	return runGUIWithDeps(args, stdout, stderr, guiCommandDeps{
		newHandler:  newServerHandler,
		listen:      net.Listen,
		serve:       serveUntilShutdown,
		openBrowser: openBrowser,
	})
}

type guiCommandDeps struct {
	newHandler  func(string, net.Addr, bool, server.LifecycleOptions) (http.Handler, string, error)
	listen      func(string, string) (net.Listener, error)
	serve       func(net.Listener, http.Handler, <-chan struct{}) error
	openBrowser func(string) error
	// launchURL, when non-nil, replaces the default guiLaunchURL. The
	// difftool/mergetool commands use it to add BASE, an output path, logical
	// labels, and the three-way mode while reusing the same serve lifecycle.
	launchURL func(baseURL, token string) string
}

// guiSessionRequest is one blocking browser session. The command that opens a
// comparison fills how to bind and what URL to open; serveGUISession owns the
// listen, lifecycle lease, and shutdown so `gui`, `difftool`, and `mergetool`
// all wait for the tab identically.
type guiSessionRequest struct {
	addr         string
	noOpen       bool
	allowRemote  bool
	launchURL    func(baseURL, token string) string
	mergeOutcome func(output string, unresolved int)
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
		fmt.Fprintln(fs.Output(), `ayame-diff gui [--addr host:port] [--allow-remote] [--no-open] [LEFT [RIGHT]]

Start the local web UI and open it in your browser. Same UI as `+"`serve`"+`, but
picks a free localhost port and launches the browser for you. With two paths,
the GUI chooses text/folder mode and starts comparing immediately. Non-loopback
addresses require the explicit --allow-remote safety opt-in.`)
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}
	if code, done := parseFlagsOrExit(fs, args, stdout, stderr); done {
		return code
	}
	if fs.NArg() > 2 {
		fmt.Fprintln(stderr, "error: gui accepts at most two paths: LEFT RIGHT")
		return exitUsage
	}
	paths := fs.Args()
	if deps.launchURL == nil {
		deps.launchURL = func(baseURL, token string) string {
			return guiLaunchURL(baseURL, paths, token)
		}
	}
	return serveGUISession(guiSessionRequest{
		addr: addr, noOpen: noOpen, allowRemote: allowRemote, launchURL: deps.launchURL,
	}, deps, stderr)
}

// serveGUISession binds, serves, and blocks until the browser tab closes (its
// lifecycle lease expires), the user stops the server, or a signal arrives. It
// returns the process exit code for that session.
func serveGUISession(req guiSessionRequest, deps guiCommandDeps, stderr io.Writer) int {
	remote, err := remoteBind(req.addr)
	if err != nil {
		reportError(stderr, err)
		return exitUsage
	}
	if remote && !req.allowRemote {
		fmt.Fprintln(stderr, "error: non-loopback listen addresses require --allow-remote")
		return exitUsage
	}

	// Listen first: the Host allowlist and the launch URL both need the port
	// actually bound, which the default "port 0" only reveals here.
	ln, portFallback, err := listenWithPortFallback(deps.listen, "tcp", req.addr)
	if err != nil {
		reportError(stderr, err)
		return exitError
	}
	defer ln.Close()
	if portFallback {
		fmt.Fprintf(stderr, "warning: %s is unavailable; using %s\n", req.addr, ln.Addr())
	}
	shutdownRequests, requestShutdown := newShutdownRequest()
	handler, token, err := deps.newHandler(version, ln.Addr(), remote, server.LifecycleOptions{
		Shutdown:            requestShutdown,
		BrowserLeaseTimeout: guiBrowserLeaseTimeout,
		BrowserCloseGrace:   guiBrowserCloseGrace,
		MergeOutcome:        req.mergeOutcome,
	})
	if err != nil {
		reportError(stderr, err)
		return exitError
	}
	if remote {
		printRemoteWarning(stderr)
	}
	guiURL := req.launchURL(browserBaseURL(ln.Addr()), token)
	fmt.Fprintf(stderr, "ayame-diff GUI at %s  (Stop server or Ctrl+C)\n", guiURL)
	if !req.noOpen {
		if err := deps.openBrowser(guiURL); err != nil {
			fmt.Fprintf(stderr, "could not open a browser automatically (%v); open %s manually\n", err, guiURL)
		}
	}
	if err := deps.serve(ln, handler, shutdownRequests); err != nil && !errors.Is(err, http.ErrServerClosed) {
		reportError(stderr, err)
		return exitError
	}
	return exitOK
}

// guiLaunchURL builds the URL the browser opens. The token always rides along:
// it is how the page comes to hold the credential the API requires (#108).
func guiLaunchURL(base string, paths []string, token string) string {
	query := url.Values{}
	if token != "" {
		query.Set("token", token)
	}
	if len(paths) == 0 {
		if len(query) == 0 {
			return base
		}
		return base + "?" + query.Encode()
	}
	query.Set("old", paths[0])
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
