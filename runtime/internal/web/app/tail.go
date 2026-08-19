package app

import (
	"net/http"
	"strconv"
	"strings"

	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

func handleSessionEvents(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/session/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "events" {
		http.NotFound(w, r)
		return
	}
	slug := parts[0]
	after := 0
	if raw := r.URL.Query().Get("after"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeBadAfter(w)
			return
		}
		after = n
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
	for _, row := range rows {
		if err := tmpl.ExecuteTemplate(w, "event-row", row); err != nil {
			writeTemplateError(w)
			return
		}
	}
}

func isNotFoundErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
