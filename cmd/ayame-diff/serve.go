package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/hjosugi/ayame-diff/internal/server"
)

// runServe implements: ayame-diff serve [--addr host:port]
//
// It starts a local web UI for browsing diffs. The server binds to localhost by
// default; it reads the file paths given in the browser, so it is meant for
// local single-user use only.
func runServe(args []string, stdout, stderr io.Writer) int {
	return runServeWithDeps(args, stdout, stderr, serveCommandDeps{
		newHandler:     newServerHandler,
		listenAndServe: http.ListenAndServe,
	})
}

type serveCommandDeps struct {
	newHandler     func(string) (http.Handler, error)
	listenAndServe func(string, http.Handler) error
}

func runServeWithDeps(args []string, stdout, stderr io.Writer, deps serveCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff serve", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
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
			return exitOK
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	handler, err := deps.newHandler(version)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	fmt.Fprintf(stderr, "ayame-diff serving on http://%s  (Ctrl+C to stop)\n", addr)
	if err := deps.listenAndServe(addr, handler); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	return exitOK
}

func newServerHandler(version string) (http.Handler, error) {
	srv, err := server.New(version)
	if err != nil {
		return nil, err
	}
	return srv.Handler(), nil
}
