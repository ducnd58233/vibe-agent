package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// EventLogPath is the event log for a slug.
func EventLogPath(workspaceRoot, slug string) string {
	return filepath.Join(RunDir(workspaceRoot, slug), EventLogName)
}

// AppendEvent adds one line to the log and returns the stored event, including
// the sequence number it was given.
//
// Sequence is derived by counting existing lines rather than tracked in memory,
// so two processes writing the same log cannot silently agree on a number.
// Concurrent writers can still collide; the runner is single-writer per run.
func AppendEvent(path string, event Event) (Event, error) {
	if event.Type == "" {
		return Event{}, errors.New("event type must not be empty")
	}
	if event.At.IsZero() {
		event.At = time.Now()
	}
	event.At = event.At.UTC()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Event{}, fmt.Errorf("create run directory: %w", err)
	}

	existing, err := countLines(path)
	if err != nil {
		return Event{}, err
	}
	event.Sequence = existing + 1

	encoded, err := json.Marshal(event)
	if err != nil {
		return Event{}, fmt.Errorf("encode event: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Event{}, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return Event{}, fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		return Event{}, fmt.Errorf("sync event log: %w", err)
	}
	return event, nil
}

// ReadEvents returns every event in the log. A missing log is not an error: a
// run that has recorded nothing yet has no events.
func ReadEvents(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(text, &event); err != nil {
			return nil, fmt.Errorf("event log %s line %d is not valid JSON: %w", path, line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read event log: %w", err)
	}
	return events, nil
}

func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read event log: %w", err)
	}
	return count, nil
}
