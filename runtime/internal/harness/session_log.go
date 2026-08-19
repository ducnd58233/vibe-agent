package harness

import (
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// appendSession stores one gesture on every active run, or on the workspace
// ambient log when no run is in flight. Errors are dropped: session bookkeeping
// must not wedge a coding session.
func appendSession(req Request, record session.Record) {
	if record.Source == "" {
		record.Source = session.SourceHook
	}
	record.Client = string(req.Client)
	if record.At.IsZero() {
		record.At = time.Now().UTC()
	}

	runs := activeRuns(req.WorkspaceRoot)
	if len(runs) == 0 {
		_, _ = session.Append(session.AmbientLogPath(req.WorkspaceRoot), record)
		return
	}
	for _, run := range runs {
		_, _ = session.Append(session.LogPath(req.WorkspaceRoot, run.Slug), record)
	}
}

func recordSessionStart(req Request) {
	appendSession(req, session.Record{
		Type:  session.TypeSessionStart,
		Event: string(EventSessionStart),
	})
}

func recordPromptSubmit(req Request, body payload) {
	appendSession(req, session.Record{
		Type: session.TypePromptSubmit,
		Body: body.text(),
	})
}

func recordPreToolUse(req Request, body payload) {
	tool := body.sessionTool()
	command := body.sessionCommand()
	if emptyToolRow(tool, command) {
		return
	}
	appendSession(req, session.Record{
		Type:    session.TypePreTool,
		Event:   string(EventPreToolUse),
		Tool:    tool,
		Command: command,
	})
}

func recordStop(req Request, body payload, subagent bool) {
	event := EventStop
	typ := session.TypeStop
	if subagent {
		event = EventSubagentStop
		typ = session.TypeSubagentStop
	}
	appendSession(req, session.Record{
		Type:  typ,
		Event: string(event),
	})
	if body.LastAssistantMessage != "" {
		appendSession(req, session.Record{
			Type: session.TypeTranscriptMessage,
			Role: "assistant",
			Body: body.LastAssistantMessage,
		})
	}
	projectTranscript(req, body, body.LastAssistantMessage)
}

func recordToolUse(req Request, body payload) {
	tool := body.sessionTool()
	command := body.sessionCommand()
	if emptyToolRow(tool, command) {
		return
	}
	appendSession(req, session.Record{
		Type:    session.TypeToolUse,
		Tool:    tool,
		Command: command,
	})
}

func emptyToolRow(tool, command string) bool {
	return strings.TrimSpace(tool) == "" && strings.TrimSpace(command) == ""
}
