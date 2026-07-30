package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/ayame-editor/ayame-diff/internal/server"
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
		serve:      serveUntilShutdown,
	})
}

type serveCommandDeps struct {
	newHandler func(string, net.Addr, bool, server.LifecycleOptions) (http.Handler, string, error)
	listen     func(string, string) (net.Listener, error)
	serve      func(net.Listener, http.Handler, <-chan struct{}) error
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

	// Listen first: the Host allowlist and the printed URL both need the port
	// actually bound, which "port 0" only reveals here.
	ln, err := deps.listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	defer ln.Close()
	shutdownRequests, requestShutdown := newShutdownRequest()
	handler, token, err := deps.newHandler(version, ln.Addr(), remote, server.LifecycleOptions{Shutdown: requestShutdown})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	if remote {
		printRemoteWarning(stderr)
	}
	// The URL carries the API token; without it the browser cannot call the
	// API at all, so print the whole thing (#108).
	fmt.Fprintf(stderr, "ayame-diff serving on %s  (Stop server or Ctrl+C)\n", tokenURL(browserBaseURL(ln.Addr()), token))
	if err := deps.serve(ln, handler, shutdownRequests); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stderr, "error:", err)
		return exitError
	}
	return exitOK
}

// newServerHandler builds the server for a bound listener, returning its API
// token so the caller can put it in the URL it prints or opens.
//
// A loopback listener gets an exact Host allowlist, which is what defeats DNS
// rebinding. A deliberately remote listener does not: the names it is reachable
// under are not knowable here, so the token alone guards it (#108).
func newServerHandler(version string, addr net.Addr, remote bool, lifecycle server.LifecycleOptions) (http.Handler, string, error) {
	opts := server.Options{Version: version, Lifecycle: lifecycle}
	if !remote {
		opts.AllowedHosts = loopbackHosts(addr)
	}
	srv, err := server.NewWithOptions(opts)
	if err != nil {
		return nil, "", err
	}
	return srv.Handler(), srv.Token(), nil
}
