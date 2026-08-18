package app

import (
	"net/http"
	"net/url"
	"strings"
)

func handleNewSession(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	goal := strings.TrimSpace(r.FormValue("goal"))
	graphID := strings.TrimSpace(r.FormValue("graph"))
	if err := StartDeliveryRun(d.workspaceRoot, d.toolkitRoot, slug, goal, graphID); err != nil {
		http.Error(w, "could not start session", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/session/"+url.PathEscape(slug), http.StatusSeeOther)
}

func handleComposerSend(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/session/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "send" {
		http.NotFound(w, r)
		return
	}
	slug := parts[0]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	hostID := strings.TrimSpace(r.FormValue("host"))
	message := strings.TrimSpace(r.FormValue("message"))
	if err := SendComposerMessage(r.Context(), d.workspaceRoot, slug, hostID, message); err != nil {
		http.Error(w, "send failed", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/session/"+url.PathEscape(slug), http.StatusSeeOther)
}
