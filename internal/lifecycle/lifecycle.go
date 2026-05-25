// Package lifecycle owns the daemon shutdown sequence: once the reviewer
// finalizes the session, linger briefly so the browser can render the
// success state, then gracefully shut down the HTTP server.
package lifecycle

import (
	"context"
	"net/http"
	"time"
)

// Options tunes the linger and graceful-shutdown windows.
type Options struct {
	Linger          time.Duration
	ShutdownTimeout time.Duration
}

// WaitFinalizeThenShutdown blocks until `done` is closed, then lingers for
// opts.Linger, then gracefully shuts down srv with opts.ShutdownTimeout.
// If ctx is canceled first, the linger is skipped and the function returns
// ctx.Err() (after attempting a best-effort shutdown).
func WaitFinalizeThenShutdown(ctx context.Context, done <-chan struct{}, srv *http.Server, opts Options) error {
	select {
	case <-done:
		// Finalized; linger then shut down cleanly.
		if opts.Linger > 0 {
			timer := time.NewTimer(opts.Linger)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				_ = shutdown(srv, opts.ShutdownTimeout)
				return ctx.Err()
			}
		}
		return shutdown(srv, opts.ShutdownTimeout)

	case <-ctx.Done():
		_ = shutdown(srv, opts.ShutdownTimeout)
		return ctx.Err()
	}
}

func shutdown(srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return srv.Shutdown(ctx)
}
