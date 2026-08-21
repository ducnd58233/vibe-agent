package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// SSE handlers type-assert the ResponseWriter to http.Flusher before
// streaming. AccessLog must not silently drop that capability just because it
// wraps the writer to capture a status code.
func TestAccessLogPreservesFlusher(t *testing.T) {
	var flushable bool
	h := AccessLog(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, flushable = w.(http.Flusher)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/demo/events/stream", nil)
	h.ServeHTTP(rec, req)
	if !flushable {
		t.Fatal("ResponseWriter passed to the handler must still satisfy http.Flusher after AccessLog wraps it")
	}
}
