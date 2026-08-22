package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventLogName is the append-only log beside the manifest.
const EventLogName = "events.ndjson"

// EventType is the closed set of run-log kinds. Writers must use these
// constants; AppendEvent rejects anything outside the set.
type EventType string

const (
	EventRunStarted  EventType = "run_started"
	EventTransition  EventType = "transition"
	EventFlagSet     EventType = "flag_set"
	EventToolUse     EventType = "tool_use"
	EventRunAborted  EventType = "run_aborted"
	EventRunResumed  EventType = "run_resumed"
	EventRunExtended EventType = "run_extended"
)

var knownEventTypes = map[EventType]struct{}{
	EventRunStarted:  {},
	EventTransition:  {},
	EventFlagSet:     {},
	EventToolUse:     {},
	EventRunAborted:  {},
	EventRunResumed:  {},
	EventRunExtended: {},
}

// Valid reports whether t is a known run event kind.
func (t EventType) Valid() bool {
	_, ok := knownEventTypes[t]
	return ok
}

// Event is one recorded fact about a run: a node entered, a verifier finished,
// a human approved. Checks point at events by Ref, so the log is where evidence
// actually lives.
//
// The log is append-only. Rewriting it would let a later run erase what an
// earlier one recorded, which is the opposite of what evidence is for.
type Event struct {
	Sequence int             `json:"sequence"`
	Type     EventType       `json:"type"`
	Node     string          `json:"node,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	At       time.Time       `json:"at"`
}

// Ref is the pointer form a check stores, for example "events.ndjson#41".
func (e Event) Ref() string {
	return fmt.Sprintf("%s#%d", EventLogName, e.Sequence)
}
