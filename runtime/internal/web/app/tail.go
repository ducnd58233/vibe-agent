package app

import (
	"net/http"
	"strings"

	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

func handleSessionEvents(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	slug, ok := parseSessionSubpath(r.URL.Path, "events")
	if !ok {
		http.NotFound(w, r)
		return
	}
	after, ok := parseAfterQuery(r)
	if !ok {
		writeBadAfter(w)
		return
	}
	selectedView := r.URL.Query().Get("view")
	rows, err := view.EventsAfterForView(d.activeWorkspace(r), slug, after, selectedView)
	if err != nil {
		if isNotFoundErr(err) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<p class="empty">Could not read the session log.</p>`))
		return
	}
	tmpl, err := ui.Templates()
	if err != nil {
		writeTemplateError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := writeEventRows(w, tmpl, rows); err != nil {
		writeTemplateError(w)
		return
	}
}

func isNotFoundErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
