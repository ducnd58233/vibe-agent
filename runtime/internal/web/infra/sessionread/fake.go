package sessionread

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// Fake implements Reader with in-memory data for tests.
type Fake struct {
	Events  map[string][]session.Event
	Ambient AmbientStat
	Hosts   map[string]string
}

func (f Fake) Replay(workspaceRoot, slug string) ([]session.Event, error) {
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

func (f Fake) AmbientStat(_ string) AmbientStat {
	return f.Ambient
}

func (f Fake) PeekHost(logPath string) string {
	if f.Hosts == nil {
		return ""
	}
	return f.Hosts[logPath]
}
