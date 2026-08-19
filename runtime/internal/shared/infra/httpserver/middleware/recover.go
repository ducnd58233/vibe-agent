package middleware

import (
	"fmt"
	"net/http"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

const msgInternal = "internal server error"

// Recover turns a panic into a logged 500 instead of a crashed process.
func Recover(l observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if x := recover(); x != nil {
					observability.LogErrorContext(r.Context(), l, "panic recovered",
						fmt.Errorf("%T: %v", x, x))
					http.Error(w, msgInternal, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
