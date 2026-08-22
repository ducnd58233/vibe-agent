// Package session stores typed host gestures in an append-only log.
//
// Delivery-graph events live in run/domain; session gestures are a separate
// ledger the web UI and CLI replay.
package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LogName is the append-only session file beside a run manifest.
const LogName = "session.ndjson"

// Type is a closed set of session event kinds.
type Type string

const (
	TypeSessionStart Type = "session_start"
	TypePromptSubmit Type = "prompt_submit"
	TypePreTool      Type = "pre_tool"
	TypeToolUse      Type = "tool_use"
	TypeStop         Type = "stop"
	TypeSubagentStop Type = "subagent_stop"
	// TypeMessage is projected or printed chat text (assistant, user, …).
	TypeMessage Type = "message"
	// TypeThinking is host reasoning that must not enter the compose prefix.
	TypeThinking Type = "thinking"
)

// legacyTypeTranscriptMessage is the pre-split wire value. Writers never emit
// it; parseEvent remaps it by role so leftover local logs still render.
const legacyTypeTranscriptMessage Type = "transcript_message"

var knownTypes = map[Type]struct{}{
	TypeSessionStart: {},
	TypePromptSubmit: {},
	TypePreTool:      {},
	TypeToolUse:      {},
	TypeStop:         {},
	TypeSubagentStop: {},
	TypeMessage:      {},
	TypeThinking:     {},
}

func (t Type) valid() bool {
	_, ok := knownTypes[t]
	return ok
}

// TypeForChatRole picks message vs thinking for a transcript or print role.
func TypeForChatRole(role string) Type {
	if strings.EqualFold(strings.TrimSpace(role), "thinking") {
		return TypeThinking
	}
	return TypeMessage
}

// NormalizeType remaps a stored type. Legacy transcript_message lines become
// thinking or message from their payload role.
func NormalizeType(t Type, role string) Type {
	if t != legacyTypeTranscriptMessage {
		return t
	}
	return TypeForChatRole(role)
}

// Source is where the gesture was captured.
type Source string

const (
	SourceHook       Source = "hook"
	SourceTranscript Source = "transcript"
	SourceGraph      Source = "graph"
	SourcePrint      Source = "print"
)

// FilterKind is the UI filter bucket for a row.
type FilterKind string

const (
	FilterHook       FilterKind = "hook"
	FilterTool       FilterKind = "tool"
	FilterSkill      FilterKind = "skill"
	FilterGraph      FilterKind = "graph"
	FilterTranscript FilterKind = "transcript"
)

// Usage holds host-reported token fields when the caller already has them.
type Usage struct {
	Input     int `json:"input,omitempty"`
	Output    int `json:"output,omitempty"`
	CacheRead int `json:"cacheRead,omitempty"`
	Total     int `json:"total,omitempty"`
}

// Record is one session gesture before it is written.
type Record struct {
	Type    Type
	Source  Source
	Client  string
	Role    string
	Event   string
	Tool    string
	Command string
	Body    string
	Usage   *Usage
	At      time.Time
}

// Event is a stored session line with sequence metadata.
type Event struct {
	Sequence int             `json:"sequence"`
	Type     Type            `json:"type"`
	Source   Source          `json:"source"`
	Client   string          `json:"client,omitempty"`
	Role     string          `json:"role,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	At       time.Time       `json:"at"`
}

// Payload is the redacted body stored under Event.Payload.
type Payload struct {
	Source  Source `json:"source"`
	Client  string `json:"client,omitempty"`
	Role    string `json:"role,omitempty"`
	Event   string `json:"event,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Command string `json:"command,omitempty"`
	Body    string `json:"body,omitempty"`
	Usage   *Usage `json:"usage,omitempty"`
}

func (r Record) validate() error {
	if !r.Type.valid() {
		return fmt.Errorf("session: unknown type %q", r.Type)
	}
	switch r.Source {
	case SourceHook, SourceTranscript, SourceGraph, SourcePrint:
	default:
		return fmt.Errorf("session: unknown source %q", r.Source)
	}
	return nil
}

func (r Record) payload() (Payload, error) {
	if err := r.validate(); err != nil {
		return Payload{}, err
	}
	return Payload{
		Source:  r.Source,
		Client:  r.Client,
		Role:    r.Role,
		Event:   RedactText(r.Event),
		Tool:    r.Tool,
		Command: RedactText(TruncateCommand(r.Command)),
		Body:    RedactText(r.Body),
		Usage:   r.Usage,
	}, nil
}
