package sse

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const DefaultPollInterval = time.Second

const headerAccelBuffering = "X-Accel-Buffering"

// Conn holds a live SSE response. Call Flush after each WriteEvent.
type Conn struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// ErrStreamingUnsupported means the ResponseWriter cannot flush incrementally.
var ErrStreamingUnsupported = errors.New("streaming unsupported")

// Begin sets SSE headers and returns a connection wrapper.
func Begin(w http.ResponseWriter) (*Conn, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, ErrStreamingUnsupported
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set(headerAccelBuffering, "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &Conn{w: w, flusher: flusher}, nil
}

// WriteEvent writes one frame and flushes it to the client.
func (c *Conn) WriteEvent(e Event) error {
	if err := WriteEvent(c.w, e); err != nil {
		return err
	}
	c.flusher.Flush()
	return nil
}

// Poll calls produce on each tick until ctx is cancelled or produce returns an error.
func Poll(ctx context.Context, c *Conn, interval time.Duration, produce func(context.Context) ([]Event, error)) error {
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			events, err := produce(ctx)
			if err != nil {
				return err
			}
			for _, e := range events {
				if err := c.WriteEvent(e); err != nil {
					return err
				}
			}
		}
	}
}
