package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/hjosugi/ayame-diff/internal/server"
)

// runServe implements: ayame-diff serve [--addr host:port] [--allow-remote]
//
// It starts a local web UI for browsing diffs. The server binds to localhost by
// default; it reads the file paths given in the browser, so it is meant for
// local single-user use only.
func runServe(args []string, stdout, stderr io.Writer) int {
	return runServeWithDeps(args, stdout, stderr, serveCommandDeps{
		newHandler: newServerHandler,
		listen:     net.Listen,
		serve: func(ln net.Listener, handler http.Handler) error {
			return server.NewHTTPServer("", handler).Serve(ln)
		},
	})
}

type serveCommandDeps struct {
	newHandler func(string) (http.Handler, error)
	listen     func(string, string) (net.Listener, error)
	serve      func(net.Listener, http.Handler) error
}

func runServeWithDeps(args []string, stdout, stderr io.Writer, deps serveCommandDeps) int {
	fs := flag.NewFlagSet("ayame-diff serve", flag.ContinueOnError)
	fs.SetOutput(flagOutput(args, stdout, stderr))
	var addr string
	var allowRemote bool
	fs.StringVar(&addr, "addr", "127.0.0.1:8080", "listen address (host:port)")
	fs.BoolVar(&allowRemote, "allow-remote", false, "allow a non-loopback listen address (unsafe without network access controls)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `ayame-diff serve [--addr host:port] [--allow-remote]

Start a local web UI for comparing files in the browser. Binds to localhost by
default; it opens the paths you type, so run it only for your own local use.
Non-loopback addresses require the explicit --allow-remote safety opt-in.`)
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
	fmt.Fprintf(stderr, "ayame-diff serving on %s  (Ctrl+C to stop)\n", browserBaseURL(ln.Addr()))
	if err := deps.serve(ln, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
