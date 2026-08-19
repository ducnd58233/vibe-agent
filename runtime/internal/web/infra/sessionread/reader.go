package sessionread

import (
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// AmbientStat is filesystem metadata for the ambient session journal.
type AmbientStat struct {
	Present bool
	Size    int64
	ModTime time.Time
}

// Reader loads session NDJSON logs and related metadata for the web UI.
type Reader interface {
	Replay(workspaceRoot, slug string) ([]session.Event, error)
	AmbientStat(workspaceRoot string) AmbientStat
	PeekHost(logPath string) string
}
