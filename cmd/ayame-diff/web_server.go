package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ayame-editor/ayame-diff/internal/server"
)

const (
	guiBrowserLeaseTimeout = 90 * time.Second
	guiBrowserCloseGrace   = 5 * time.Second
)

// A variable so a focused test can exercise the close escalation without
// waiting out the real drain budget.
var gracefulShutdownTimeout = 5 * time.Second

func newShutdownRequest() (<-chan struct{}, func()) {
	requests := make(chan struct{}, 1)
	request := func() {
		select {
		case requests <- struct{}{}:
		default:
		}
	}
	return requests, request
}

func listenWithPortFallback(
	listen func(string, string) (net.Listener, error),
	network string,
	address string,
) (net.Listener, bool, error) {
	listener, err := listen(network, address)
	if err == nil || !isPortConflict(err) {
		return listener, false, err
	}

	host, portText, splitErr := net.SplitHostPort(address)
	if splitErr != nil {
		return nil, false, err
	}
	port, parseErr := strconv.Atoi(portText)
	if parseErr != nil || port == 0 {
		return nil, false, err
	}
	for candidatePort := port + 1; candidatePort <= 65535; candidatePort++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(candidatePort))
		listener, candidateErr := listen(network, candidate)
		if candidateErr == nil {
			return listener, true, nil
		}
		if !isPortConflict(candidateErr) {
			return nil, false, candidateErr
		}
		err = candidateErr
	}
	return nil, false, err
}

func serveUntilShutdown(listener net.Listener, handler http.Handler, shutdownRequests <-chan struct{}) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveUntilContext(ctx, listener, handler, shutdownRequests)
}

func serveUntilContext(ctx context.Context, listener net.Listener, handler http.Handler, shutdownRequests <-chan struct{}) error {
	httpServer := server.NewHTTPServer("", handler)
	// Requests hang off one cancellable context so a stop can end the waiting
	// ones itself. The GUI keeps an external-change watch open for up to twenty
	// seconds (#251); without this the graceful drain would always outlive its
	// deadline and report a stop as a failure (#323).
	requests, endRequests := context.WithCancel(context.Background())
	defer endRequests()
	httpServer.BaseContext = func(net.Listener) context.Context { return requests }

	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serveErrors:
		return err
	case <-ctx.Done():
	case <-shutdownRequests:
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	endRequests()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		// The deadline is the designed escalation to an immediate close, not
		// something to report: the process is stopping either way.
		_ = httpServer.Close()
		if !errors.Is(err, context.DeadlineExceeded) {
			<-serveErrors
			return err
		}
	}
	serveErr := <-serveErrors
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}
