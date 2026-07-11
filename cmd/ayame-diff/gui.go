package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/hjosugi/ayame-diff/internal/server"
)

// runGUI implements: ayame-diff gui [--addr host:port] [--no-open]
//
// It starts the same local web UI as `serve` but, by default, binds an
// ephemeral localhost port and opens the browser — the "double-click to a GUI"
// experience without a native webview dependency (keeping the single static,
// cross-compiled binary). See ADR 0002 / hjosugi/ayame-diff#14.
func runGUI(args []string) {
	fs := flag.NewFlagSet("ayame-diff gui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var addr string
	var noOpen bool
	fs.StringVar(&addr, "addr", "127.0.0.1:0", "listen address; port 0 picks a free port")
	fs.BoolVar(&noOpen, "no-open", false, "start the server but do not open the browser")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff gui [--addr host:port] [--no-open]

Start the local web UI and open it in your browser. Same UI as `+"`serve`"+`, but
picks a free localhost port and launches the browser for you.`)
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

	srv, err := server.New(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Fprintf(os.Stderr, "ayame-diff GUI at %s  (Ctrl+C to stop)\n", url)
	if !noOpen {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "could not open a browser automatically (%v); open %s manually\n", err, url)
		}
	}
	if err := http.Serve(ln, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// openBrowser opens url in the platform's default browser. The launcher is
// detached; a failure to start is reported to the caller.
func openBrowser(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	return exec.Command(name, args...).Start()
}
