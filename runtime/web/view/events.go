package view

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

const (
	chatFoldRunes = 480
	chatFoldLines = 8
)

// KindOrder is the pipeline and filter menu order.
var KindOrder = []session.FilterKind{
	session.FilterHook,
	session.FilterTool,
	session.FilterSkill,
	session.FilterGraph,
	session.FilterTranscript,
}

// EventRow is one rendered trajectory line.
type EventRow struct {
	Seq         int
	Role        string
	Kind        session.FilterKind
	Source      session.Source
	Type        session.Type
	Client      string
	Tool        string
	Command     string
	EventName   string
	At          time.Time
	Summary     string
	Body        string
	BodyHTML    template.HTML
	PayloadJSON string
	Usage       *session.Usage
	HasUsage    bool
	TokensText  string
	Failed      bool
	HostGap     bool
	Redacted    bool
	SearchText  string
	FoldClosed  bool
}

type payloadView struct {
	session.Payload
	Failed   bool `json:"failed,omitempty"`
	ExitCode *int `json:"exitCode,omitempty"`
	HostGap  bool `json:"hostGap,omitempty"`
}

// ProjectEvents maps stored session lines to UI rows.
func ProjectEvents(events []session.Event) []EventRow {
	rows := make([]EventRow, 0, len(events))
	for _, ev := range events {
		row := projectEvent(ev)
		rows = append(rows, row)
	}
	promoteUsageToAssistant(rows)
	return rows
}

// promoteUsageToAssistant moves host usage off trace-only rows onto the
// last assistant in that turn so Chat and Trajectory both paint in/out/cache.
func promoteUsageToAssistant(rows []EventRow) {
	assistant := -1
	for i := range rows {
		switch rows[i].Role {
		case "user":
			assistant = -1
		case "assistant":
			assistant = i
		}
		if !rows[i].HasUsage || ChatVisibleRole(rows[i].Role) {
			continue
		}
		if assistant < 0 || rows[assistant].HasUsage {
			continue
		}
		rows[assistant].Usage = rows[i].Usage
		rows[assistant].HasUsage = true
		rows[assistant].TokensText = rows[i].TokensText
		rows[i].Usage = nil
		rows[i].HasUsage = false
		rows[i].TokensText = ""
	}
}

func projectEvent(ev session.Event) EventRow {
	var body payloadView
	if len(ev.Payload) > 0 {
		_ = json.Unmarshal(ev.Payload, &body)
	}
	if body.Client == "" {
		body.Client = ev.Client
	}
	if body.Role == "" {
		body.Role = ev.Role
	}
	if body.Source == "" {
		body.Source = ev.Source
	}
	kind := session.Kind(ev)
	role := eventRole(ev, body.Payload)
	summary := eventSummary(ev, body.Payload)
	displayBody := eventBody(ev, body.Payload)
	if strings.TrimSpace(displayBody) == strings.TrimSpace(summary) {
		displayBody = ""
	}
	row := EventRow{
		Seq:       ev.Sequence,
		Role:      role,
		Kind:      kind,
		Source:    body.Source,
		Type:      ev.Type,
		Client:    body.Client,
		Tool:      body.Tool,
		Command:   body.Command,
		EventName: body.Event,
		At:        ev.At,
		Summary:   summary,
		Body:      displayBody,
		BodyHTML:  markdownBody(role, displayBody),
		Failed:    body.Failed,
		Redacted: strings.Contains(displayBody, "[REDACTED]") ||
			strings.Contains(displayBody, "<credential>"),
	}
	if body.Usage.Reported() {
		row.Usage = body.Usage
		row.HasUsage = true
		row.TokensText = formatUsage(*body.Usage)
	}
	if len(ev.Payload) > 0 {
		row.PayloadJSON = string(ev.Payload)
	}
	row.HostGap = body.HostGap
	row.FoldClosed = chatFoldClosed(role, displayBody)
	row.SearchText = strings.ToLower(strings.Join([]string{
		summary,
		displayBody,
		role,
		string(kind),
		fmt.Sprint(ev.Sequence),
	}, " "))
	return row
}

func eventRole(ev session.Event, body session.Payload) string {
	if body.Source == session.SourceGraph {
		return "context"
	}
	switch ev.Type {
	case session.TypeSessionStart, session.TypeStop, session.TypeSubagentStop:
		return "system"
	case session.TypePromptSubmit:
		if session.LooksLikeComposePrefix(body.Body) {
			return "hook"
		}
		return "user"
	case session.TypeTranscriptMessage:
		if role := strings.ToLower(strings.TrimSpace(body.Role)); role != "" {
			return role
		}
		return "assistant"
	case session.TypePreTool:
		return "hook"
	case session.TypeToolUse:
		return "tool"
	default:
		return "system"
	}
}

func eventSummary(ev session.Event, body session.Payload) string {
	switch ev.Type {
	case session.TypeSessionStart:
		if body.Event == "ComposerStart" {
			if body.Client != "" {
				return "Composer start · client " + body.Client
			}
			return "Composer start"
		}
		if body.Client != "" {
			return "SessionStart · client " + body.Client
		}
		return "SessionStart"
	case session.TypePromptSubmit:
		return "UserPromptSubmit"
	case session.TypePreTool:
		if body.Tool != "" {
			return body.Tool
		}
		return "PreToolUse"
	case session.TypeToolUse:
		if body.Tool != "" {
			return body.Tool
		}
		return "ToolUse"
	case session.TypeStop:
		switch body.Event {
		case "SessionEnd":
			return "SessionEnd"
		case "ComposerStop":
			if body.Client != "" {
				return "Composer stop · client " + body.Client
			}
			return "Composer stop"
		}
		return "Stop"
	case session.TypeSubagentStop:
		return "SubagentStop"
	case session.TypeTranscriptMessage:
		if strings.ToLower(strings.TrimSpace(body.Role)) == "thinking" {
			return "thinking"
		}
		if body.Role != "" {
			return "projected " + strings.ToLower(body.Role) + " text"
		}
		return "transcript message"
	default:
		if body.Event != "" {
			return body.Event
		}
		return string(ev.Type)
	}
}

func eventBody(ev session.Event, body session.Payload) string {
	switch ev.Type {
	case session.TypeToolUse, session.TypePreTool:
		if body.Command != "" {
			return body.Command
		}
		if body.Body != "" {
			return body.Body
		}
		return ""
	}
	if body.Body != "" {
		return body.Body
	}
	if body.Command != "" {
		return body.Command
	}
	return body.Event
}

// KindCounts tallies filter buckets for the menu.
func KindCounts(rows []EventRow) map[session.FilterKind]int {
	counts := make(map[session.FilterKind]int)
	for _, row := range rows {
		counts[row.Kind]++
	}
	return counts
}

// ChatRows returns the operator thread: user, thinking, and assistant.
func ChatRows(rows []EventRow) []EventRow {
	out := make([]EventRow, 0, len(rows))
	for _, row := range rows {
		if row.Role == "thinking" && strings.TrimSpace(row.Body) == "" {
			continue
		}
		if ChatVisibleRole(row.Role) {
			out = append(out, row)
		}
	}
	return out
}

func chatFoldClosed(role, body string) bool {
	if role == "thinking" {
		return strings.TrimSpace(body) != ""
	}
	if role != "user" {
		return false
	}
	if strings.Count(body, "\n") >= chatFoldLines {
		return true
	}
	return utf8.RuneCountInString(body) > chatFoldRunes
}

// ChatVisibleRole is the Chat tab allow-list. Trajectory keeps the full
// trace (system, hook, tool, graph, question, context). Thinking sits above
// the assistant reply and starts collapsed.
func ChatVisibleRole(role string) bool {
	switch role {
	case "user", "assistant", "thinking":
		return true
	default:
		return false
	}
}

// ChatHasProse reports whether Chat would show any rows.
func ChatHasProse(rows []EventRow) bool {
	return len(ChatRows(rows)) > 0
}
