package app

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
)

type slugExistsResponse struct {
	Exists bool `json:"exists"`
}

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
		httpserver.JSON(w, http.StatusOK, slugExistsResponse{Exists: false})
		return
	}
	manifest := state.ManifestPath(d.activeWorkspace(r), slug)
	_, err := os.Stat(filepath.Clean(manifest))
	httpserver.JSON(w, http.StatusOK, slugExistsResponse{Exists: err == nil})
}

func handleComposerSend(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	slug, ok := parseSessionSubpath(r.URL.Path, "send")
	if !ok {
		http.NotFound(w, r)
		return
	}
	sessionURL := "/session/" + url.PathEscape(slug) + "?view=chat"
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, sessionURL+"&error=bad+form", http.StatusSeeOther)
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
		http.Redirect(w, r, sessionURL+"&error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, sessionURL, http.StatusSeeOther)
}
