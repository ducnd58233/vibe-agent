package persistence

import (
	"encoding/json"
	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendEventNumbersSequentiallyFromOne(t *testing.T) {
	_, path := indexedPaths(t, t.TempDir(), "slug")

	for i := 0; i < 3; i++ {
		if _, err := AppendEvent(path, domain.Event{Type: "node_entered", Node: "build", At: fixedTime()}); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}

	events, err := ReadEvents(path)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	for i, event := range events {
		if event.Sequence != i+1 {
			t.Errorf("event %d has sequence %d, want %d", i, event.Sequence, i+1)
		}
	}
}

// The log is evidence. Rewriting it would let a later run erase what an
// earlier one recorded, which defeats the point of provenance.
func TestAppendEventOnlyAppends(t *testing.T) {
	_, path := indexedPaths(t, t.TempDir(), "slug")

	if _, err := AppendEvent(path, domain.Event{Type: "first", At: fixedTime()}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	firstPass, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if _, err := AppendEvent(path, domain.Event{Type: "second", At: fixedTime()}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	secondPass, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.HasPrefix(string(secondPass), string(firstPass)) {
		t.Error("appending rewrote earlier content instead of adding to it")
	}
}

func TestAppendEventWritesOneJSONObjectPerLine(t *testing.T) {
	_, path := indexedPaths(t, t.TempDir(), "slug")

	payload := json.RawMessage(`{"exitCode":0,"command":"go test ./..."}`)
	if _, err := AppendEvent(path, domain.Event{Type: "verifier_ran", Node: "test", Payload: payload, At: fixedTime()}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if _, err := AppendEvent(path, domain.Event{Type: "node_entered", Node: "e2e", At: fixedTime()}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), raw)
	}
	for i, line := range lines {
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Errorf("line %d is not a JSON object: %v", i, err)
		}
	}
}

func TestAppendEventRejectsAnEmptyType(t *testing.T) {
	_, path := indexedPaths(t, t.TempDir(), "slug")
	if _, err := AppendEvent(path, domain.Event{At: fixedTime()}); err == nil {
		t.Error("AppendEvent accepted an event with no type")
	}
}

func TestAppendEventReturnsTheStoredEvent(t *testing.T) {
	_, path := indexedPaths(t, t.TempDir(), "slug")
	stored, err := AppendEvent(path, domain.Event{Type: "node_entered", Node: "build", At: fixedTime()})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if stored.Sequence != 1 {
		t.Errorf("Sequence = %d, want 1", stored.Sequence)
	}
	if stored.Ref() != "events.ndjson#1" {
		t.Errorf("Ref() = %q, want events.ndjson#1", stored.Ref())
	}
}

func TestAppendEventDefaultsMissingTimestamp(t *testing.T) {
	_, path := indexedPaths(t, t.TempDir(), "slug")
	before := time.Now().UTC().Add(-time.Second)
	stored, err := AppendEvent(path, domain.Event{Type: "node_entered"})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if stored.At.Before(before) {
		t.Errorf("At = %v, want a real timestamp", stored.At)
	}
}

func TestReadEventsOnMissingLogReturnsNothing(t *testing.T) {
	events, err := ReadEvents(filepath.Join(t.TempDir(), "never-written", "events.ndjson"))
	if err != nil {
		t.Fatalf("ReadEvents on a missing log should not error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}
