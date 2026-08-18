package view

import (
	"fmt"
	"os"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// EventsAfter returns trajectory rows with sequence greater than after.
func EventsAfter(workspaceRoot, slug string, after int) ([]EventRow, error) {
	var logPath string
	switch slug {
	case "ambient":
		logPath = session.AmbientLogPath(workspaceRoot)
	default:
		logPath = session.LogPath(workspaceRoot, slug)
	}
	events, err := session.Replay(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("log not found: %w", err)
		}
		return nil, err
	}
	rows := ProjectEvents(events)
	if after <= 0 {
		return rows, nil
	}
	out := make([]EventRow, 0)
	for _, row := range rows {
		if row.Seq > after {
			out = append(out, row)
		}
	}
	return out, nil
}

// LastSequence returns the highest event sequence in the log, or 0 when empty.
func LastSequence(workspaceRoot, slug string) int {
	rows, err := EventsAfter(workspaceRoot, slug, 0)
	if err != nil || len(rows) == 0 {
		return 0
	}
	return rows[len(rows)-1].Seq
}
