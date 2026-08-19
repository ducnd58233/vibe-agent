package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
)

func TestRequestIDGeneratesAndEchoes(t *testing.T) {
	var got string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = httpserver.RequestIDFrom(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	if got == "" {
		t.Fatal("missing request id in context")
	}
	if rec.Header().Get("X-Request-Id") != got {
		t.Fatalf("header %q != context %q", rec.Header().Get("X-Request-Id"), got)
	}
}

func TestRequestIDHonoursInbound(t *testing.T) {
	const want = "trace-abc"
	var got string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = httpserver.RequestIDFrom(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", want)
	h.ServeHTTP(rec, req)
	if got != want {
		t.Fatalf("context id = %q, want %q", got, want)
	}
}
