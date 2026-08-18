package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

// WorkspacesPath is `.agent-state/web-workspaces.json` for the primary workspace.
func WorkspacesPath(primaryWorkspace string) string {
	return filepath.Join(workspace.StateDir(primaryWorkspace), domain.WorkspacesFileName())
}

// WriteWorkspaces persists the registry beside web.json.
func WriteWorkspaces(primaryWorkspace string, reg domain.Registry) error {
	if err := os.MkdirAll(workspace.StateDir(primaryWorkspace), 0o750); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	encoded, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	path := WorkspacesPath(primaryWorkspace)
	temp := path + ".tmp"
	if err := os.WriteFile(temp, encoded, 0o600); err != nil {
		return fmt.Errorf("write workspaces: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("replace workspaces: %w", err)
	}
	return nil
}

// LoadWorkspaces reads a saved registry when present.
func LoadWorkspaces(primaryWorkspace string) (domain.Registry, bool, error) {
	path := WorkspacesPath(primaryWorkspace)
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Registry{}, false, nil
		}
		return domain.Registry{}, false, fmt.Errorf("read workspaces: %w", err)
	}
	var reg domain.Registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return domain.Registry{}, true, fmt.Errorf("parse workspaces: %w", err)
	}
	return reg, true, nil
}
