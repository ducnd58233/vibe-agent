package app

import (
	"html"
	"net/http"
	"strings"
)

const htmxEmptyErrorClass = "empty"

// writeHTMXFragment writes a safe HTML partial for HTMX swaps.
// Non-HTMX callers get the same fragment with text/html (callers that need
// a full page should use renderError instead).
func writeHTMXFragment(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<p class="` + htmxEmptyErrorClass + `">` + html.EscapeString(message) + `</p>`))
}

// writeHTMXOrError picks HTMX fragment vs http.Error for legacy partial paths.
func writeHTMXOrError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if isHTMX(r) {
		w.WriteHeader(status)
		writeHTMXFragment(w, message)
		return
	}
	http.Error(w, message, status)
}

func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}
