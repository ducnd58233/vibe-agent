package sessionread

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// FS reads session logs through the session package, which resolves a path to
// its rows in memory.db (or a legacy file not yet migrated).
type FS struct{}

// NewFS returns a filesystem-backed session reader.
func NewFS() FS {
	return FS{}
}

func logPath(workspaceRoot, slug string) string {
	if slug == "ambient" {
		return session.AmbientLogPath(workspaceRoot)
	}
	return session.LogPath(workspaceRoot, slug)
}

// Replay loads session gestures for slug ("ambient" uses the ambient journal).
func (FS) Replay(workspaceRoot, slug string) ([]session.Event, error) {
	events, err := session.Replay(logPath(workspaceRoot, slug))
	if err != nil && session.IsNotFound(err) {
		return nil, nil
	}
	return events, err
}

// AmbientStat reports whether the ambient journal has any events. Rows live in
// memory.db, so the timestamp is the last event's rather than a file's mtime.
func (FS) AmbientStat(workspaceRoot string) AmbientStat {
	events, err := session.Replay(session.AmbientLogPath(workspaceRoot))
	if err != nil || len(events) == 0 {
		return AmbientStat{}
	}
	return AmbientStat{
		Present: true,
		Size:    int64(len(events)),
		ModTime: events[len(events)-1].At,
	}
}

// PeekHost returns payload.client from the first events that carry one.
func (FS) PeekHost(logPath string) string {
	events, err := session.Replay(logPath)
	if err != nil {
		return ""
	}
	for i := 0; i < len(events) && i < 8; i++ {
		if events[i].Client != "" {
			return events[i].Client
		}
	}
	return ""
}
