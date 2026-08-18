package view

import (
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

// WorkspaceRow is one registered workspace in the sidebar.
type WorkspaceRow struct {
	ID     string
	Label  string
	Path   string
	Active bool
}

// ProjectWorkspaces maps registry roots to sidebar rows.
func ProjectWorkspaces(reg domain.Registry, activeRoot string) []WorkspaceRow {
	active := filepath.Clean(activeRoot)
	rows := make([]WorkspaceRow, 0, len(reg.Roots))
	for _, root := range reg.Roots {
		rows = append(rows, WorkspaceRow{
			ID:     reg.ID(root),
			Label:  filepath.Base(root),
			Path:   root,
			Active: root == active,
		})
	}
	return rows
}
