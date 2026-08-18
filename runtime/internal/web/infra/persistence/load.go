package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

// LoadState reads web.json when present.
func LoadState(workspaceRoot string) (domain.State, bool, error) {
	path := StatePath(workspaceRoot)
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return domain.State{}, false, nil
		}
		return domain.State{}, false, fmt.Errorf("read web state: %w", err)
	}
	var state domain.State
	if err := json.Unmarshal(raw, &state); err != nil {
		return domain.State{}, true, fmt.Errorf("parse web state: %w", err)
	}
	return state, true, nil
}
