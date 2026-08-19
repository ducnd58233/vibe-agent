package app

import (
	"net/http"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/catalog"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
)

func (d httpDeps) handleCatalogCommands(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	d.renderCatalog(w, r, catalog.FamilyCommand)
}

func (d httpDeps) handleCatalogSkills(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	d.renderCatalog(w, r, catalog.FamilySkill)
}

func (d httpDeps) renderCatalog(w http.ResponseWriter, r *http.Request, family catalog.Family) {
	idx, err := catalog.LoadForWorkspace(d.activeWorkspace(r), d.toolkitRoot)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusInternalServerError, "catalog error")
		return
	}
	q := r.URL.Query().Get("q")
	var items []catalog.Entry
	switch family {
	case catalog.FamilyCommand:
		items = idx.SearchCommands(q)
	case catalog.FamilySkill:
		items = idx.SearchSkills(q)
	}
	tmpl, err := ui.Templates()
	if err != nil {
		writeTemplateError(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "catalog-items", items); err != nil {
		writeTemplateError(w, r)
	}
}
