package middleware

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
)

const (
	maxRequestIDLen = 128
	headerRequestID = "X-Request-Id"
)

// RequestID honours inbound X-Request-Id or generates one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := clipRequestID(strings.TrimSpace(r.Header.Get(headerRequestID)))
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(headerRequestID, id)
		next.ServeHTTP(w, r.WithContext(httpserver.WithRequestID(r.Context(), id)))
	})
}

func clipRequestID(id string) string {
	if utf8.RuneCountInString(id) <= maxRequestIDLen {
		return id
	}
	return string([]rune(id)[:maxRequestIDLen])
}
