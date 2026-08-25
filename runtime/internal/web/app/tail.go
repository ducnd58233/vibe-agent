package app

import (
	"net/http"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

func handleSessionEvents(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	slug, ok := parseSessionSubpath(r.URL.Path, "events")
	if !ok {
		writeNotFound(w, r)
		return
	}
	after, ok := parseAfterQuery(r)
	if !ok {
		writeBadAfter(w, r)
		return
	}
	selectedView := r.URL.Query().Get("view")
	rows, err := view.EventsAfterForView(d.sessionRead, d.activeWorkspace(r), slug, after, selectedView)
	if err != nil {
		if session.IsNotFound(err) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			return
		}
		writeHTMXFragment(w, "Could not read the session log.")
		return
	}
	tmpl, err := ui.Templates()
	if err != nil {
		writeTemplateError(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := writeEventRows(w, tmpl, rows); err != nil {
		writeTemplateError(w, r)
		return
	}
}
