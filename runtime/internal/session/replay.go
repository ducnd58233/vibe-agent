package session

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

const (
	DefaultReplayTurns = 8
	DefaultReplayBytes = 16384
)

func decodePayload(raw json.RawMessage, out *Payload) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// Replay reads session events in sequence order. Unknown types are skipped
// without inventing fields.
func Replay(logPath string) ([]Event, error) {
	lines, err := state.ReadEvents(logPath)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(lines))
	for _, line := range lines {
		ev, ok, err := parseEvent(line)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out, nil
}

func parseEvent(line state.Event) (Event, bool, error) {
	t := Type(line.Type)
	if !t.valid() {
		return Event{}, false, nil
	}
	var body Payload
	if len(line.Payload) > 0 {
		if err := json.Unmarshal(line.Payload, &body); err != nil {
			return Event{}, false, fmt.Errorf("session event #%d: %w", line.Sequence, err)
		}
	}
	return Event{
		Sequence: line.Sequence,
		Type:     t,
		Source:   body.Source,
		Client:   body.Client,
		Role:     body.Role,
		Payload:  line.Payload,
		At:       line.At,
	}, true, nil
}

func eventPayload(ev Event) Payload {
	var body Payload
	_ = json.Unmarshal(ev.Payload, &body)
	return body
}

// ComposePrefixFromLog builds a redacted Chat prefix from prior turns.
func ComposePrefixFromLog(logPath string) string {
	events, err := Replay(logPath)
	if err != nil {
		return ""
	}
	return ComposePrefix(events, DefaultReplayTurns, DefaultReplayBytes)
}

// ComposePrefix keeps the last user/assistant/question turns, dropping oldest
// when the turn or byte cap is exceeded.
func ComposePrefix(events []Event, maxTurns, maxBytes int) string {
	var lines []string
	for _, ev := range events {
		body := strings.TrimSpace(eventPayload(ev).Body)
		if body == "" {
			continue
		}
		switch ev.Type {
		case TypePromptSubmit:
			lines = append(lines, "User: "+body)
		case TypeTranscriptMessage:
			if EphemeralHostStatus(body) {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(eventPayload(ev).Role))
			if role == "" {
				role = strings.ToLower(strings.TrimSpace(ev.Role))
			}
			switch role {
			case "assistant":
				lines = append(lines, "Assistant: "+body)
			case "question":
				lines = append(lines, "Question: "+body)
			}
		}
	}
	if maxTurns > 0 {
		lines = lastUserTurns(lines, maxTurns)
	}
	text := strings.Join(lines, "\n")
	for maxBytes > 0 && len(text) > maxBytes && len(lines) > 0 {
		lines = lines[1:]
		text = strings.Join(lines, "\n")
	}
	return text
}

// HasPromptSubmitBody reports whether the log already ends with the same user prompt.
// Web composer records a prompt before the host user-prompt-submit hook fires.
func HasPromptSubmitBody(logPath, body string) bool {
	events, err := Replay(logPath)
	if err != nil {
		return false
	}
	trimmed := strings.TrimSpace(body)
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type != TypePromptSubmit {
			continue
		}
		if strings.TrimSpace(eventPayload(events[i]).Body) == trimmed {
			return true
		}
	}
	return false
}

// LooksLikeComposePrefix reports spawn memory that must not become a Chat user card.
func LooksLikeComposePrefix(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "User:") && strings.Contains(trimmed, "\nAssistant:")
}

func lastUserTurns(lines []string, n int) []string {
	users := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "User:") {
			users++
			if users == n {
				return lines[i:]
			}
		}
	}
	return lines
}
