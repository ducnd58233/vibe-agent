package app

import (
	"html"
	"net/http"
	"strings"

	ui "github.com/ducnd58233/vibe-agent/runtime/web"
)

const htmxEmptyErrorClass = "empty"

// writeHTMXFragment writes a safe HTML partial for HTMX swaps.
// Non-HTMX callers get the same fragment with text/html (callers that need
// a full page should use renderError instead).
func writeHTMXFragment(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<p class="` + htmxEmptyErrorClass + `">` + html.EscapeString(message) + `</p>`))
}

// writeHTMXOrError picks HTMX fragment vs a full-page renderError for partial paths.
func writeHTMXOrError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if isHTMX(r) {
		w.WriteHeader(status)
		writeHTMXFragment(w, message)
		return
	}
	tmpl, err := ui.Templates()
	if err != nil {
		// Templates unavailable: still avoid bare http.Error for browser pages.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte("<!doctype html><title>Error</title><p>" + html.EscapeString(message) + "</p>"))
		return
	}
	title := http.StatusText(status)
	if title == "" {
		title = "Error"
	}
	renderError(w, tmpl, status, title, message)
}

func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

// writeNotFound renders a 404 page (or HTMX fragment) without bare http.NotFound.
func writeNotFound(w http.ResponseWriter, r *http.Request) {
	writeHTMXOrError(w, r, http.StatusNotFound, "The page you are looking for does not exist.")
}
