package lifecycle

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// newServerOnRandomPort starts an HTTP server on a random port and returns
// it along with the listener so the test can close it.
func newServerOnRandomPort(t *testing.T) (*http.Server, net.Listener) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	return srv, ln
}

func TestWaitFinalizeThenShutdown_lingersThenShuts(t *testing.T) {
	srv, _ := newServerOnRandomPort(t)
	done := make(chan struct{})

	start := time.Now()
	errCh := make(chan error, 1)
	go func() {
		errCh <- WaitFinalizeThenShutdown(context.Background(), done, srv, Options{
			Linger:          80 * time.Millisecond,
			ShutdownTimeout: 1 * time.Second,
		})
	}()

	close(done)

	select {
	case err := <-errCh:
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if elapsed < 70*time.Millisecond {
			t.Errorf("did not linger; elapsed = %v", elapsed)
		}
		if elapsed > 500*time.Millisecond {
			t.Errorf("lingered too long; elapsed = %v", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}
}

func TestWaitFinalizeThenShutdown_ctxCancelSkipsLinger(t *testing.T) {
	srv, _ := newServerOnRandomPort(t)
	done := make(chan struct{}) // never closed
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- WaitFinalizeThenShutdown(ctx, done, srv, Options{
			Linger:          10 * time.Second,
			ShutdownTimeout: 1 * time.Second,
		})
	}()

	cancel()

	select {
	case err := <-errCh:
		// Expect context error wrapped or returned.
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ctx-cancel should skip the 10s linger")
	}
}

func TestWaitFinalizeThenShutdown_zeroLingerWorks(t *testing.T) {
	srv, _ := newServerOnRandomPort(t)
	done := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- WaitFinalizeThenShutdown(context.Background(), done, srv, Options{
			Linger:          0,
			ShutdownTimeout: 1 * time.Second,
		})
	}()

	close(done)
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("zero linger errored: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out")
	}
}
