package middleware

import (
	"net/http"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/redact"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush lets SSE handlers stream through AccessLog: wrapping ResponseWriter in
// a struct hides http.Flusher unless the wrapper forwards it explicitly.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// AccessLog records one line per request with status and duration.
func AccessLog(l observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			if l == nil {
				return
			}
			l.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", redact.Text(r.URL.Path),
				"request_id", httpserver.RequestIDFrom(r.Context()),
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
