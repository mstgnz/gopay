// Package server runs the HTTP server through its whole life, including the part that used to
// be missing: draining in-flight requests before the process exits.
//
// On a payment gateway that is not cosmetic. A deploy sends SIGTERM to every replica in turn,
// and a request cut in the middle of a provider call leaves a payment whose outcome nobody
// knows: the money may have moved while the caller was told the request failed.
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Run listens on srv.Addr and serves until ctx is cancelled, then drains.
func Run(ctx context.Context, srv *http.Server, drainTimeout time.Duration) (time.Duration, error) {
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return drainTimeout, err
	}

	return Serve(ctx, srv, listener, drainTimeout)
}

// Serve serves on listener until ctx is cancelled, then stops accepting new connections and
// waits up to drainTimeout for the requests already in flight to finish.
//
// It returns when the drain is over, along with how much of drainTimeout was left. The caller
// that has more to drain afterwards, such as background work, spends the remainder rather than a
// second full budget: the platform kills the process at the end of one grace period, not two.
//
// The error is nil after a clean drain, ctx.Err() when the timeout ran out first, or the serve
// error when the server never started.
func Serve(ctx context.Context, srv *http.Server, listener net.Listener, drainTimeout time.Duration) (time.Duration, error) {
	serveErr := make(chan error, 1)

	go func() {
		err := srv.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		// The server stopped on its own, which at this point only happens on failure.
		return drainTimeout, err
	case <-ctx.Done():
	}

	// A fresh context: the one that just got cancelled cannot time the drain.
	deadline := time.Now().Add(drainTimeout)
	drainCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	err := srv.Shutdown(drainCtx)

	return max(time.Until(deadline), 0), err
}
