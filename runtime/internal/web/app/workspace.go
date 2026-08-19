package app

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/persistence"
)

const workspaceCookie = "vibe_ws"

func (d httpDeps) activeWorkspace(r *http.Request) string {
	reg := d.snapshotRegistry()
	if cookie, err := r.Cookie(workspaceCookie); err == nil {
		if root, ok := reg.Resolve(cookie.Value); ok {
			return root
		}
	}
	return reg.Default
}

func handleWorkspaceSwitch(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeBadForm(w, r)
		return
	}
	id := strings.TrimSpace(r.FormValue("workspace_id"))
	if _, ok := d.snapshotRegistry().Resolve(id); !ok {
		http.Redirect(w, r, "/?error=unknown+workspace", http.StatusSeeOther)
		return
	}
	setWorkspaceCookie(w, id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleWorkspaceOpen(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeBadForm(w, r)
		return
	}
	raw := strings.TrimSpace(r.FormValue("path"))
	if raw == "" {
		http.Redirect(w, r, "/?error=path+required", http.StatusSeeOther)
		return
	}
	abs, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		http.Redirect(w, r, "/?error=bad+path", http.StatusSeeOther)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		http.Redirect(w, r, "/?error=workspace+directory+not+found", http.StatusSeeOther)
		return
	}
	current := d.snapshotRegistry()
	next := domain.NewRegistry(current.Default, append(append([]string{}, current.Roots...), abs))
	if err := persistence.WriteWorkspaces(next.Default, next); err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("could not save workspace"), http.StatusSeeOther)
		return
	}
	d.storeRegistry(next)
	setWorkspaceCookie(w, next.ID(abs))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func setWorkspaceCookie(w http.ResponseWriter, id string) {
	//nolint:gosec // G124 loopback server uses HTTP without TLS
	http.SetCookie(w, &http.Cookie{
		Name:     workspaceCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // loopback HTTP only
		MaxAge:   60 * 60 * 24 * 30,
	})
}

func loadRegistry(primary string, extra []string) (domain.Registry, error) {
	reg := domain.NewRegistry(primary, extra)
	saved, ok, err := persistence.LoadWorkspaces(primary)
	if err != nil {
		return reg, err
	}
	if ok {
		reg = domain.NewRegistry(reg.Default, append(saved.Roots, reg.Roots...))
	}
	if err := persistence.WriteWorkspaces(primary, reg); err != nil {
		return reg, err
	}
	return reg, nil
}
