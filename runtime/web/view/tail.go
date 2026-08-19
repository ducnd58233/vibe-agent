package view

import (
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/sessionread"
)

// EventsAfter returns trajectory rows with sequence greater than after.
func EventsAfter(logs sessionread.Reader, workspaceRoot, slug string, after int) ([]EventRow, error) {
	rows, err := TrajectoryRows(logs, workspaceRoot, slug)
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
func EventsAfterForView(logs sessionread.Reader, workspaceRoot, slug string, after int, selectedView string) ([]EventRow, error) {
	rows, err := EventsAfter(logs, workspaceRoot, slug, after)
	if err != nil {
		return nil, err
	}
	if NormalizeSessionView(selectedView) == "chat" {
		demoteIntermediateAssistants(rows)
	}
	return rows, nil
}

// TrajectoryRows is the Trajectory tab: session gestures plus graph transitions.
func TrajectoryRows(logs sessionread.Reader, workspaceRoot, slug string) ([]EventRow, error) {
	events, err := logs.Replay(workspaceRoot, slug)
	if err != nil {
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
func LastSequence(logs sessionread.Reader, workspaceRoot, slug string) int {
	rows, err := EventsAfter(logs, workspaceRoot, slug, 0)
	if err != nil || len(rows) == 0 {
		return 0
	}
	return rows[len(rows)-1].Seq
}
