package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/hjosugi/ayame-diff/internal/server"
)

// runServe implements: ayame-diff serve [--addr host:port]
//
// It starts a local web UI for browsing diffs. The server binds to localhost by
// default; it reads the file paths given in the browser, so it is meant for
// local single-user use only.
func runServe(args []string) {
	fs := flag.NewFlagSet("ayame-diff serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var addr string
	fs.StringVar(&addr, "addr", "127.0.0.1:8080", "listen address (host:port)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff serve [--addr host:port]

Start a local web UI for comparing files in the browser. Binds to localhost by
default; it opens the paths you type, so run it only for your own local use.`)
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
	fmt.Fprintf(os.Stderr, "ayame-diff serving on http://%s  (Ctrl+C to stop)\n", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
