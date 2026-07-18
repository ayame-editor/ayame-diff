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
)

// runGUI implements: ayame-diff gui [--addr host:port] [--no-open] [OLD [NEW]]
//
// It starts the same local web UI as `serve` but, by default, binds an
// ephemeral localhost port and opens the browser — the "double-click to a GUI"
// experience without a native webview dependency (keeping the single static,
// cross-compiled binary). See ADR 0002 / hjosugi/ayame-diff#14.
func runGUI(args []string, stdout, stderr io.Writer) int {
	return runGUIWithDeps(args, stdout, stderr, guiCommandDeps{
		newHandler:  newServerHandler,
		listen:      net.Listen,
		serve:       http.Serve,
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
	fs.StringVar(&addr, "addr", "127.0.0.1:0", "listen address; port 0 picks a free port")
	fs.BoolVar(&noOpen, "no-open", false, "start the server but do not open the browser")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff gui [--addr host:port] [--no-open] [OLD [NEW]]

Start the local web UI and open it in your browser. Same UI as `+"`serve`"+`, but
picks a free localhost port and launches the browser for you. With two paths,
the GUI chooses text/folder mode and starts comparing immediately.`)
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
	guiURL := guiLaunchURL("http://"+ln.Addr().String()+"/", fs.Args())
	fmt.Fprintf(stderr, "ayame-diff GUI at %s  (Ctrl+C to stop)\n", guiURL)
	if !noOpen {
		if err := deps.openBrowser(guiURL); err != nil {
			fmt.Fprintf(stderr, "could not open a browser automatically (%v); open %s manually\n", err, guiURL)
		}
	}
	if err := deps.serve(ln, handler); err != nil {
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
