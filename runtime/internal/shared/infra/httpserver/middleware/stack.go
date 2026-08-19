package middleware

import (
	"net/http"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

// StandardStack wraps h with request ID, optional access log, and panic recovery.
func StandardStack(h http.Handler, log observability.Logger) http.Handler {
	if log == nil {
		return Chain(h, RequestID)
	}
	return Chain(h, RequestID, AccessLog(log), Recover(log))
}
