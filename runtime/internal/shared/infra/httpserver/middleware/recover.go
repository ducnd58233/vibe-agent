package middleware

import (
	"context"
	"net/http"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

const msgInternal = "internal server error"

// Recover turns a panic into a logged 500 instead of a crashed process.
func Recover(l observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func(ctx context.Context) {
				if x := recover(); x != nil {
					observability.LogPanicRecovered(ctx, l, x)
					httpserver.RespondError(w, r, http.StatusInternalServerError, msgInternal)
				}
			}(r.Context())
			next.ServeHTTP(w, r)
		})
	}
}
