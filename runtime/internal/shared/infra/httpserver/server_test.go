package httpserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

func TestServeReturnsBindError(t *testing.T) {
	ctx := context.Background()
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	t.Cleanup(func() { _ = ln.Close() })

	err = Serve(ctx, addr, http.NewServeMux(), observability.Discard())
	if err == nil {
		t.Fatal("expected listen error when address is already bound")
	}
}

func TestServeShutsDownOnCancel(t *testing.T) {
	ctx := context.Background()
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	serveCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(serveCtx, addr, http.NewServeMux(), observability.Discard())
	}()

	deadline := time.Now().Add(2 * time.Second)
	dialer := &net.Dialer{Timeout: 50 * time.Millisecond}
	dialCtx := context.Background()
	for {
		c, dialErr := dialer.DialContext(dialCtx, "tcp", addr)
		if dialErr == nil {
			_ = c.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("server never accepted connections: %v", dialErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve = %v, want nil after cancel", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}
