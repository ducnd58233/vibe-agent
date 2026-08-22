package view

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

func TestProjectRunGraphEventsMapsStartAndTransition(t *testing.T) {
	stamp := time.Date(2026, 8, 19, 2, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(graphTransitionPayload{From: "intake", To: "research", Via: "spec_ready=true"})
	if err != nil {
		t.Fatal(err)
	}
	rows := ProjectRunGraphEvents([]state.Event{
		{Sequence: 1, Type: state.EventRunStarted, Node: "intake", At: stamp, Payload: []byte(`{"goal":"show graph rows","graph":"goal-delivery"}`)},
		{Sequence: 2, Type: state.EventToolUse, Node: "research", At: stamp.Add(time.Second), Payload: []byte(`{"tool":"Bash","command":"ls"}`)},
		{Sequence: 3, Type: state.EventTransition, Node: "research", At: stamp.Add(2 * time.Second), Payload: payload},
		{Sequence: 4, Type: state.EventTransition, Node: "research", At: stamp.Add(3 * time.Second), Payload: mustJSON(t, graphTransitionPayload{From: "research", To: "research", Via: "(fallback)"})},
	})
	if len(rows) != 3 {
		t.Fatalf("rows = %d want 3 (journal tool_use skipped)", len(rows))
	}
	if rows[0].Kind != session.FilterGraph || rows[0].Role != "graph" || rows[0].Source != session.SourceGraph {
		t.Fatalf("start row = %+v", rows[0])
	}
	if rows[0].Summary != "Run started" || rows[0].Body != "show graph rows" {
		t.Fatalf("start copy = %q %q want goal, not node id", rows[0].Summary, rows[0].Body)
	}
	if ChatVisibleRole(rows[0].Role) {
		t.Fatal("graph rows must stay off Chat")
	}
	if rows[1].Summary != "research" || rows[1].Body != "from intake via spec_ready=true" {
		t.Fatalf("transition copy = %q %q", rows[1].Summary, rows[1].Body)
	}
	if rows[2].Summary != "research" || rows[2].Body != "" {
		t.Fatalf("fallback via must be omitted, got body %q", rows[2].Body)
	}
}

func TestProjectRunStartedRedactsGoalSecret(t *testing.T) {
	stamp := time.Date(2026, 8, 19, 2, 0, 0, 0, time.UTC)
	secret := "sk-0123456789abcdef0123456789ab"
	rows := ProjectRunGraphEvents([]state.Event{
		{Sequence: 1, Type: state.EventRunStarted, Node: "intake", At: stamp, Payload: []byte(`{"goal":"ship with ` + secret + `"}`)},
	})
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if strings.Contains(rows[0].Body, secret) {
		t.Fatalf("secret leaked in body %q", rows[0].Body)
	}
	if strings.Contains(rows[0].PayloadJSON, secret) {
		t.Fatalf("secret leaked in payload %q", rows[0].PayloadJSON)
	}
}

func TestProjectRunStartedFallsBackToNodeWhenGoalMissing(t *testing.T) {
	stamp := time.Date(2026, 8, 19, 2, 0, 0, 0, time.UTC)
	rows := ProjectRunGraphEvents([]state.Event{
		{Sequence: 1, Type: state.EventRunStarted, Node: "intake", At: stamp, Payload: []byte(`{"graph":"goal-delivery"}`)},
	})
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Body != "intake" {
		t.Fatalf("body = %q want node id when goal is missing", rows[0].Body)
	}
}

func TestMergeTrajectoryRenumbersByTime(t *testing.T) {
	stamp := time.Date(2026, 8, 19, 2, 0, 0, 0, time.UTC)
	sessionRows := []EventRow{
		{Seq: 1, Kind: session.FilterHook, Role: "user", At: stamp.Add(time.Minute), Summary: "prompt"},
	}
	graphRows := []EventRow{
		{Seq: 1, Kind: session.FilterGraph, Role: "graph", At: stamp, Summary: "Run started"},
		{Seq: 2, Kind: session.FilterGraph, Role: "graph", At: stamp.Add(2 * time.Minute), Summary: "build"},
	}
	merged := MergeTrajectory(sessionRows, graphRows)
	if len(merged) != 3 {
		t.Fatalf("merged = %d", len(merged))
	}
	if merged[0].Summary != "Run started" || merged[0].Seq != 1 {
		t.Fatalf("first = %+v", merged[0])
	}
	if merged[1].Summary != "prompt" || merged[1].Seq != 2 {
		t.Fatalf("second = %+v", merged[1])
	}
	if merged[2].Summary != "build" || merged[2].Seq != 3 {
		t.Fatalf("third = %+v", merged[2])
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
