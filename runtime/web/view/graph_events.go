package view

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/redact"
)

type graphTransitionPayload struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via"`
}

type graphStartPayload struct {
	Goal  string `json:"goal,omitempty"`
	Graph string `json:"graph,omitempty"`
}

// ProjectRunGraphEvents maps delivery-log lines onto Trajectory graph rows.
// Journal tool_use lines stay on the session log; they are not graph rows.
func ProjectRunGraphEvents(events []state.Event) []EventRow {
	out := make([]EventRow, 0)
	for _, ev := range events {
		row, ok := projectRunGraphEvent(ev)
		if ok {
			out = append(out, row)
		}
	}
	return out
}

func projectRunGraphEvent(ev state.Event) (EventRow, bool) {
	switch ev.Type {
	case state.EventRunStarted, state.EventTransition:
	default:
		return EventRow{}, false
	}
	summary, body, payloadJSON := graphEventCopy(ev)
	if strings.TrimSpace(body) == strings.TrimSpace(summary) {
		body = ""
	}
	row := EventRow{
		Seq:         ev.Sequence,
		Role:        "graph",
		Kind:        session.FilterGraph,
		Source:      session.SourceGraph,
		Type:        session.Type(ev.Type),
		At:          ev.At,
		Summary:     summary,
		Body:        body,
		PayloadJSON: payloadJSON,
		EventName:   string(ev.Type),
	}
	row.SearchText = strings.ToLower(strings.Join([]string{
		summary, body, "graph", ev.Node, fmt.Sprint(ev.Sequence),
	}, " "))
	return row, true
}

func graphEventCopy(ev state.Event) (summary, body, payloadJSON string) {
	node := strings.TrimSpace(ev.Node)
	payloadJSON = string(ev.Payload)
	switch ev.Type {
	case state.EventRunStarted:
		var payload graphStartPayload
		_ = json.Unmarshal(ev.Payload, &payload)
		payload.Goal = redact.Text(strings.TrimSpace(payload.Goal))
		if encoded, err := json.Marshal(payload); err == nil {
			payloadJSON = string(encoded)
		}
		if payload.Goal != "" {
			return "Run started", payload.Goal, payloadJSON
		}
		if node == "" {
			return "Run started", "", payloadJSON
		}
		return "Run started", node, payloadJSON
	case state.EventTransition:
		var payload graphTransitionPayload
		_ = json.Unmarshal(ev.Payload, &payload)
		to := strings.TrimSpace(payload.To)
		if to == "" {
			to = node
		}
		if to == "" {
			return "Graph transition", "", payloadJSON
		}
		parts := make([]string, 0, 2)
		if from := strings.TrimSpace(payload.From); from != "" && from != to {
			parts = append(parts, "from "+from)
		}
		if via := strings.TrimSpace(payload.Via); via != "" && via != "(fallback)" {
			parts = append(parts, "via "+via)
		}
		return to, strings.Join(parts, " "), payloadJSON
	default:
		return string(ev.Type), node, payloadJSON
	}
}

// MergeTrajectory interleaves session and graph rows by time and numbers Seq
// 1..n so Chat/SSE share one cursor.
func MergeTrajectory(sessionRows, graphRows []EventRow) []EventRow {
	all := make([]EventRow, 0, len(sessionRows)+len(graphRows))
	all = append(all, sessionRows...)
	all = append(all, graphRows...)
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].At.Equal(all[j].At) {
			return all[i].At.Before(all[j].At)
		}
		if all[i].Kind != all[j].Kind {
			return all[i].Kind < all[j].Kind
		}
		return all[i].Seq < all[j].Seq
	})
	for i := range all {
		all[i].Seq = i + 1
	}
	return all
}
