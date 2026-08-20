package main

import (
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// Sixteen parked runs accumulated before anyone counted them, because a parked
// run and an active one look identical in a list. These are the cases that
// decide whether the note is worth reading.
func TestIdleRunNotesOnlyWhatIsWaiting(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		status state.Status
		idle   time.Duration
		want   bool
	}{
		{"parked past the threshold", state.StatusAwaitingHuman, 4 * 24 * time.Hour, true},
		{"running past the threshold", state.StatusRunning, 5 * 24 * time.Hour, true},
		{"parked over a weekend", state.StatusAwaitingHuman, 2 * 24 * time.Hour, false},
		{"picked up an hour ago", state.StatusRunning, time.Hour, false},
		// A run that finished is finished. Saying so on every doctor run would
		// bury the ones actually waiting.
		{"closed long ago", state.StatusCancelled, 30 * 24 * time.Hour, false},
		{"done long ago", state.StatusDone, 30 * 24 * time.Hour, false},
		{"failed long ago", state.StatusFailed, 30 * 24 * time.Hour, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			run, err := state.NewRun("parked", "goal", "goal-delivery", 50, now.Add(-testCase.idle))
			if err != nil {
				t.Fatal(err)
			}
			run.Status = testCase.status
			run.CurrentNode = "approve_spec"
			run.UpdatedAt = now.Add(-testCase.idle)

			note := idleRun(run, now)
			if (note != "") != testCase.want {
				t.Fatalf("idleRun = %q, want noted = %t", note, testCase.want)
			}
			if !testCase.want {
				return
			}
			for _, part := range []string{"parked", "approve_spec", string(testCase.status)} {
				if !strings.Contains(note, part) {
					t.Errorf("the note does not contain %q: %s", part, note)
				}
			}
		})
	}
}

// A run whose manifest carries no timestamp is a shape to survive, not to
// report on: there is nothing to measure idleness against.
func TestARunWithNoTimestampIsNotCalledIdle(t *testing.T) {
	run, err := state.NewRun("undated", "goal", "goal-delivery", 50, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	run.Status = state.StatusAwaitingHuman
	run.UpdatedAt = time.Time{}

	if note := idleRun(run, time.Now()); note != "" {
		t.Errorf("a run with no timestamp was called idle: %s", note)
	}
}
