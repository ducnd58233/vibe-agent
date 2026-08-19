package app

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	"github.com/ducnd58233/vibe-agent/runtime/internal/run/infra/persistence"
)

func handleNewSession(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=bad+form", http.StatusSeeOther)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	goal := strings.TrimSpace(r.FormValue("goal"))
	graphID := strings.TrimSpace(r.FormValue("graph"))
	if err := StartDeliveryRun(d.activeWorkspace(r), d.toolkitRoot, slug, goal, graphID); err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/session/"+url.PathEscape(slug)+"?view=chat", http.StatusSeeOther)
}

func handleCheckSlug(w http.ResponseWriter, r *http.Request, d httpDeps) {
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug == "" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"exists":false}`)
		return
	}
	manifest := persistence.ManifestPath(d.activeWorkspace(r), slug)
	_, err := os.Stat(filepath.Clean(manifest))
	w.Header().Set("Content-Type", "application/json")
	if err == nil {
		_, _ = fmt.Fprint(w, `{"exists":true}`)
	} else {
		_, _ = fmt.Fprint(w, `{"exists":false}`)
	}
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
	opts := hosts.PrintOptions{
		Model: strings.TrimSpace(r.FormValue("model")),
		Mode:  strings.TrimSpace(r.FormValue("mode")),
	}
	if opts.Mode != "agent" {
		opts.Mode = ""
	}
	if err := SendComposerMessage(r.Context(), d.activeWorkspace(r), slug, hostID, message, opts); err != nil {
		http.Error(w, "send failed", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/session/"+url.PathEscape(slug)+"?view=chat", http.StatusSeeOther)
}
