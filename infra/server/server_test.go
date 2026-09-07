package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func listen(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	return listener
}

// The point of the whole package: a request that is already running when the signal arrives
// must finish. Before this, the process exited and the caller got a cut connection, which on a
// payment is the case where money moves and nobody knows.
func TestServeFinishesInFlightRequest(t *testing.T) {
	started := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "finished")
	})

	listener := listen(t)
	srv := &http.Server{Handler: mux}
	ctx, cancel := context.WithCancel(context.Background())

	type drain struct {
		remaining time.Duration
		err       error
	}
	serveDone := make(chan drain, 1)

	go func() {
		remaining, err := Serve(ctx, srv, listener, 5*time.Second)
		serveDone <- drain{remaining: remaining, err: err}
	}()

	type result struct {
		body string
		err  error
	}
	responses := make(chan result, 1)

	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/slow", listener.Addr()))
		if err != nil {
			responses <- result{err: err}
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		responses <- result{body: string(body), err: err}
	}()

	// Cancel while the handler is running, which is what SIGTERM does mid-request.
	<-started
	cancel()

	select {
	case got := <-responses:
		if got.err != nil {
			t.Fatalf("in-flight request failed instead of draining: %v", got.err)
		}
		if got.body != "finished" {
			t.Errorf("body = %q, want finished", got.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request never completed")
	}

	select {
	case got := <-serveDone:
		if got.err != nil {
			t.Errorf("Serve returned %v, want nil after a clean drain", got.err)
		}
		// The drain took about 300ms of a 5s budget, and the rest is the caller's to spend.
		if got.remaining <= 0 || got.remaining >= 5*time.Second {
			t.Errorf("remaining budget = %v, want something between 0 and the full 5s", got.remaining)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the drain")
	}
}

func TestServeStopsAcceptingAfterShutdown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})

	listener := listen(t)
	addr := listener.Addr().String()
	srv := &http.Server{Handler: mux}
	ctx, cancel := context.WithCancel(context.Background())

	serveDone := make(chan error, 1)
	go func() {
		_, err := Serve(ctx, srv, listener, 5*time.Second)
		serveDone <- err
	}()

	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("request before shutdown failed: %v", err)
	}
	_ = resp.Body.Close()

	cancel()
	if err := <-serveDone; err != nil {
		t.Fatalf("Serve returned %v", err)
	}

	if _, err := http.Get("http://" + addr); err == nil {
		t.Error("server still accepted a request after the drain finished")
	}
}

// A handler that will not finish must not hold the process past the platform's grace period.
func TestServeGivesUpAfterDrainTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	mux := http.NewServeMux()
	mux.HandleFunc("/stuck", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})

	listener := listen(t)
	srv := &http.Server{Handler: mux}
	ctx, cancel := context.WithCancel(context.Background())

	type drain struct {
		remaining time.Duration
		err       error
	}
	serveDone := make(chan drain, 1)

	go func() {
		remaining, err := Serve(ctx, srv, listener, 100*time.Millisecond)
		serveDone <- drain{remaining: remaining, err: err}
	}()

	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/stuck", listener.Addr()))
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	<-started
	cancel()

	select {
	case got := <-serveDone:
		if !errors.Is(got.err, context.DeadlineExceeded) {
			t.Errorf("Serve returned %v, want context.DeadlineExceeded", got.err)
		}
		// The whole budget went to the request drain, so there is nothing left to hand on.
		if got.remaining != 0 {
			t.Errorf("remaining budget = %v, want 0 after the timeout was spent", got.remaining)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve ignored the drain timeout")
	}
}

func TestServeReturnsStartupFailure(t *testing.T) {
	listener := listen(t)
	if err := listener.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	_, err := Serve(context.Background(), &http.Server{Handler: http.NewServeMux()}, listener, time.Second)
	if err == nil {
		t.Fatal("Serve returned nil for a listener that cannot accept")
	}
}

func TestRunReturnsListenError(t *testing.T) {
	// Port 1 is privileged, so binding it fails as a non-root test process.
	_, err := Run(context.Background(), &http.Server{Addr: "127.0.0.1:1", Handler: http.NewServeMux()}, time.Second)
	if err == nil {
		t.Fatal("Run returned nil for an address it cannot bind")
	}
}
