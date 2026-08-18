package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

const stateFileName = "web.json"

// State is written beside other derived workspace files.
type State struct {
	URL       string    `json:"url"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
}

// StatePath is `.agent-state/web.json` for a workspace.
func StatePath(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), stateFileName)
}

// WriteState records the loopback URL for a running server.
func WriteState(workspaceRoot string, state State) error {
	if err := os.MkdirAll(workspace.StateDir(workspaceRoot), 0o750); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := StatePath(workspaceRoot)
	temp := path + ".tmp"
	if err := os.WriteFile(temp, encoded, 0o600); err != nil {
		return fmt.Errorf("write web state: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("replace web state: %w", err)
	}
	return nil
}
