package domain

import (
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
}

func newTestRun(t *testing.T) *Run {
	t.Helper()
	run, err := NewRun("loop-graph-runtime", "add a control plane", "goal-delivery", 50, fixedTime())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	return run
}

func TestNewRunIDIsSortableAndCarriesSlug(t *testing.T) {
	run := newTestRun(t)
	want := "run_2026-07-29T10-00-00Z_loop-graph-runtime"
	if run.RunID != want {
		t.Errorf("RunID = %q, want %q", run.RunID, want)
	}
	if run.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", run.Status, StatusRunning)
	}
	if run.Iteration != 0 {
		t.Errorf("Iteration = %d, want 0", run.Iteration)
	}
}

func TestNewRunRejectsBadSlug(t *testing.T) {
	for _, slug := range []string{"", "Has Caps", "under_score", "trailing-", "-leading", "a/b"} {
		if _, err := NewRun(slug, "g", "goal-delivery", 50, fixedTime()); err == nil {
			t.Errorf("NewRun(%q) accepted an invalid slug", slug)
		}
	}
}

// The whole point of the provenance enum: model output must never be able to
// mark work complete. This is the load-bearing test in this package.
func TestSetCheckRejectsSourcesOutsideTheEnum(t *testing.T) {
	run := newTestRun(t)
	for _, source := range []CheckSource{"model", "", "assumed", "llm", "exit-code"} {
		err := run.SetCheck("unit", Check{Passed: true, Source: source, At: fixedTime()})
		if err == nil {
			t.Errorf("SetCheck accepted source %q", source)
		}
		if _, ok := run.Checks["unit"]; ok {
			t.Fatalf("SetCheck stored a check despite rejecting source %q", source)
		}
	}
}

func TestSetCheckAcceptsEveryRealProvenance(t *testing.T) {
	run := newTestRun(t)
	for _, source := range []CheckSource{SourceExitCode, SourceFileAssert, SourceCIAPI, SourceHumanEvent} {
		if err := run.SetCheck("unit", Check{Passed: true, Source: source, At: fixedTime()}); err != nil {
			t.Errorf("SetCheck(%q) rejected a valid source: %v", source, err)
		}
	}
}

// A skipped check is not a passed check. Guards must be able to tell them apart.
func TestSetCheckRejectsPassedAndSkippedTogether(t *testing.T) {
	run := newTestRun(t)
	err := run.SetCheck("e2e", Check{Passed: true, Skipped: true, Source: SourceExitCode, At: fixedTime()})
	if err == nil {
		t.Error("SetCheck accepted a check that is both passed and skipped")
	}
}

func TestSetCheckStampsUpdatedAt(t *testing.T) {
	run := newTestRun(t)
	before := run.UpdatedAt
	later := fixedTime().Add(time.Minute)
	if err := run.SetCheckAt("unit", Check{Passed: true, Source: SourceExitCode, At: later}, later); err != nil {
		t.Fatalf("SetCheckAt: %v", err)
	}
	if !run.UpdatedAt.After(before) {
		t.Errorf("UpdatedAt = %v, want later than %v", run.UpdatedAt, before)
	}
}
