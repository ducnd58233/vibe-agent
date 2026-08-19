package testutil

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/sessionread"
)

// SessionReadFake implements sessionread.Reader with in-memory data for tests.
type SessionReadFake struct {
	Events  map[string][]session.Event
	Ambient sessionread.AmbientStat
	Hosts   map[string]string
}

func (f SessionReadFake) Replay(workspaceRoot, slug string) ([]session.Event, error) {
	if f.Events == nil {
		return nil, nil
	}
	key := slug
	if slug != "ambient" {
		key = workspaceRoot + "/" + slug
	}
	rows, ok := f.Events[key]
	if !ok {
		return nil, nil
	}
	return append([]session.Event(nil), rows...), nil
}

func (f SessionReadFake) AmbientStat(_ string) sessionread.AmbientStat {
	return f.Ambient
}

func (f SessionReadFake) PeekHost(logPath string) string {
	if f.Hosts == nil {
		return ""
	}
	return f.Hosts[logPath]
}
