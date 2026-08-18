package session

import (
	"encoding/json"
	"fmt"
	"sort"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
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

// MustReplay is for tests; it panics on error.
func MustReplay(logPath string) []Event {
	events, err := Replay(logPath)
	if err != nil {
		panic(fmt.Sprintf("session.Replay: %v", err))
	}
	return events
}
