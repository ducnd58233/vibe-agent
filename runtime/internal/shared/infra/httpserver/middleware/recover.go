package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
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
					if l != nil {
						l.ErrorContext(ctx, "panic recovered",
							"panic_type", fmt.Sprintf("%T", x),
							"stack", session.RedactText(string(debug.Stack())),
						)
					}
					httpserver.RespondError(w, r, http.StatusInternalServerError, msgInternal)
				}
			}(r.Context())
			next.ServeHTTP(w, r)
		})
	}
}
