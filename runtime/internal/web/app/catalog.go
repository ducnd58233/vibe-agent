package app

import (
	"net/http"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/catalog"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
)

func (d httpDeps) handleCatalogCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d.renderCatalog(w, r, catalog.FamilyCommand)
}

func (d httpDeps) handleCatalogSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d.renderCatalog(w, r, catalog.FamilySkill)
}

func (d httpDeps) renderCatalog(w http.ResponseWriter, r *http.Request, family catalog.Family) {
	idx, err := catalog.Load(d.toolkitRoot)
	if err != nil {
		http.Error(w, "catalog error", http.StatusInternalServerError)
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
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "catalog-items", items); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
