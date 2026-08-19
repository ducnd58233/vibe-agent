package view

import (
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// EventsAfter returns trajectory rows with sequence greater than after.
func EventsAfter(workspaceRoot, slug string, after int) ([]EventRow, error) {
	rows, err := TrajectoryRows(workspaceRoot, slug)
	if err != nil {
		return nil, err
	}
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

// EventsAfterForView returns trajectory rows like EventsAfter, but applies
// chat-only shaping (fold intermediate assistant progress under "thinking").
func EventsAfterForView(workspaceRoot, slug string, after int, selectedView string) ([]EventRow, error) {
	rows, err := EventsAfter(workspaceRoot, slug, after)
	if err != nil {
		return nil, err
	}
	if NormalizeSessionView(selectedView) == "chat" {
		demoteIntermediateAssistants(rows)
	}
	return rows, nil
}

// TrajectoryRows is the Trajectory tab: session gestures plus graph transitions.
func TrajectoryRows(workspaceRoot, slug string) ([]EventRow, error) {
	var logPath string
	switch slug {
	case "ambient":
		logPath = session.AmbientLogPath(workspaceRoot)
	default:
		logPath = session.LogPath(workspaceRoot, slug)
	}
	events, err := session.Replay(logPath)
	if err != nil && !session.IsNotFound(err) {
		return nil, err
	}
	graphRows := []EventRow{}
	if slug != "ambient" {
		runEvents, readErr := state.ReadEvents(state.EventLogPath(workspaceRoot, slug))
		if readErr != nil {
			return nil, readErr
		}
		graphRows = ProjectRunGraphEvents(runEvents)
	}
	return MergeTrajectory(ProjectEvents(events), graphRows), nil
}

// LastSequence returns the highest event sequence in the log, or 0 when empty.
func LastSequence(workspaceRoot, slug string) int {
	rows, err := EventsAfter(workspaceRoot, slug, 0)
	if err != nil || len(rows) == 0 {
		return 0
	}
	return rows[len(rows)-1].Seq
}
