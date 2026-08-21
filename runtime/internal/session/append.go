package session

import (
	"encoding/json"
	"path/filepath"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// LogPath is the append-only session log for a slug. Empty when no run is indexed.
func LogPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, LogName)
}

// Append redacts and stores one session gesture.
func Append(logPath string, record Record) (Event, error) {
	payload, err := record.payload()
	if err != nil {
		return Event{}, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	at := record.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	stored, err := state.AppendEvent(logPath, state.Event{
		Type:    string(record.Type),
		Payload: encoded,
		At:      at,
	})
	if err != nil {
		return Event{}, err
	}
	return Event{
		Sequence: stored.Sequence,
		Type:     record.Type,
		Source:   record.Source,
		Client:   record.Client,
		Role:     record.Role,
		Payload:  encoded,
		At:       stored.At,
	}, nil
}

// AmbientLogPath is the workspace-level journal when no delivery run is active.
func AmbientLogPath(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), LogName)
}
