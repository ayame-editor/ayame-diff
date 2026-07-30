package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hjosugi/ayame-diff/internal/server"
)

const (
	gracefulShutdownTimeout = 5 * time.Second
	guiBrowserLeaseTimeout  = 90 * time.Second
	guiBrowserCloseGrace    = 5 * time.Second
)

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

func serveUntilShutdown(listener net.Listener, handler http.Handler, shutdownRequests <-chan struct{}) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveUntilContext(ctx, listener, handler, shutdownRequests)
}

func serveUntilContext(ctx context.Context, listener net.Listener, handler http.Handler, shutdownRequests <-chan struct{}) error {
	httpServer := server.NewHTTPServer("", handler)
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
	shutdownErr := httpServer.Shutdown(shutdownContext)
	if shutdownErr != nil {
		_ = httpServer.Close()
	}
	serveErr := <-serveErrors
	if shutdownErr != nil {
		return shutdownErr
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}
