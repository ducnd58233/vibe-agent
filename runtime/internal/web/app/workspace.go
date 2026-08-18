package app

import (
	"net/http"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/persistence"
)

const workspaceCookie = "vibe_ws"

func (d httpDeps) activeWorkspace(r *http.Request) string {
	if cookie, err := r.Cookie(workspaceCookie); err == nil {
		if root, ok := d.registry.Resolve(cookie.Value); ok {
			return root
		}
	}
	return d.registry.Default
}

func handleWorkspaceSwitch(w http.ResponseWriter, r *http.Request, d httpDeps) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(r.FormValue("workspace_id"))
	if _, ok := d.registry.Resolve(id); !ok {
		http.Error(w, "unknown workspace", http.StatusBadRequest)
		return
	}
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
