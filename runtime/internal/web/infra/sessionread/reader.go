package sessionread

import (
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// AmbientStat summarises the ambient session journal. Size is the event count.
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
