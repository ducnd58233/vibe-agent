// Package httpserver holds the process HTTP server and request/response helpers.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Serve listens before logging start so a bind failure is not reported as up.
// It blocks until ctx is cancelled (graceful shutdown) or the server fails.
func Serve(ctx context.Context, addr string, h http.Handler, l observability.Logger) error {
	s := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Serve(ln)
	}()
	if l != nil {
		l.Info("http server started", "address", addr)
	}

	select {
	case <-ctx.Done():
		if l != nil {
			l.Info("http server shutting down", "reason", ctx.Err())
		}
		sdCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := s.Shutdown(sdCtx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}
		if l != nil {
			l.Info("http server stopped")
		}
		return nil
	case err := <-errChan:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
