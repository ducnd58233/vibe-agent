package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventLogName is the append-only log beside the manifest.
const EventLogName = "events.ndjson"

// Event is one recorded fact about a run: a node entered, a verifier finished,
// a human approved. Checks point at events by Ref, so the log is where evidence
// actually lives.
//
// The log is append-only. Rewriting it would let a later run erase what an
// earlier one recorded, which is the opposite of what evidence is for.
type Event struct {
	Sequence int             `json:"sequence"`
	Type     string          `json:"type"`
	Node     string          `json:"node,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	At       time.Time       `json:"at"`
}

// Ref is the pointer form a check stores, for example "events.ndjson#41".
func (e Event) Ref() string {
	return fmt.Sprintf("%s#%d", EventLogName, e.Sequence)
}
