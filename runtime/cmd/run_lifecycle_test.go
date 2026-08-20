package main

import (
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// stoppedRun writes a run in a given stopped state, at a node, with a blocker.
func stoppedRun(t *testing.T, root, slug string, status state.Status) *state.Run {
	t.Helper()
	run, err := state.NewRun(slug, "lifecycle test", "goal-delivery", 50,
		time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	run.CurrentNode = "build"
	run.Status = status
	run.Iteration = 50
	run.Blockers = []state.Blocker{{Node: "build", Reason: "dependency will not install", Attempts: 3}}
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	return run
}

func reload(t *testing.T, root, slug string) *state.Run {
	t.Helper()
	run, err := state.Load(state.ManifestPath(root, slug))
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// A run that stopped and a run someone stopped look identical in the manifest a
// week later. The reason is the difference, so it is not optional.
func TestResumeAndAbortBothDemandAReason(t *testing.T) {
	root := t.TempDir()
	stoppedRun(t, root, "demo", state.StatusFailed)

	if err := runResume([]string{"--workspace", root, "--slug", "demo"}); err == nil {
		t.Error("resume accepted a run with no reason")
	}
	if err := runAbort([]string{"--workspace", root, "--slug", "demo"}); err == nil {
		t.Error("abort accepted a run with no reason")
	}
}

// A budget is raised by an explicit increment. Clearing it would turn the stop
// rule off permanently rather than extending it once.
func TestResumingABudgetStopNeedsAnExplicitIncrement(t *testing.T) {
	root := t.TempDir()
	stoppedRun(t, root, "demo", state.StatusBudgetExceeded)

	err := runResume([]string{"--workspace", root, "--slug", "demo", "--reason", "worth another pass"})
	if err == nil {
		t.Fatal("a budget stop resumed without saying by how much")
	}
	if !strings.Contains(err.Error(), "--budget") {
		t.Errorf("error = %q, want it to name --budget", err)
	}

	if err := runResume([]string{
		"--workspace", root, "--slug", "demo", "--reason", "worth another pass", "--budget", "25",
	}); err != nil {
		t.Fatal(err)
	}
	after := reload(t, root, "demo")
	if after.MaxTransitions != 75 {
		t.Errorf("maxTransitions = %d, want 75", after.MaxTransitions)
	}
	if after.Status != state.StatusRunning {
		t.Errorf("status = %s, want running", after.Status)
	}
}

// The blocker count is what ended the run. Leaving it in place would end the
// resumed run on its first failure rather than after three.
func TestResumeClearsTheBlockerThatEndedTheRun(t *testing.T) {
	root := t.TempDir()
	stoppedRun(t, root, "demo", state.StatusFailed)

	if err := runResume([]string{"--workspace", root, "--slug", "demo", "--reason", "dependency published"}); err != nil {
		t.Fatal(err)
	}
	if blockers := reload(t, root, "demo").Blockers; len(blockers) != 0 {
		t.Errorf("blockers = %+v, want none", blockers)
	}
}

// A finished run is not a stop to undo. Restarting it would reopen work its own
// evidence says is complete.
func TestAFinishedRunIsNeitherResumedNorAborted(t *testing.T) {
	root := t.TempDir()
	stoppedRun(t, root, "demo", state.StatusDone)

	if err := runResume([]string{"--workspace", root, "--slug", "demo", "--reason", "one more"}); err == nil {
		t.Error("resume restarted a done run")
	}
	if err := runAbort([]string{"--workspace", root, "--slug", "demo", "--reason", "tidying"}); err == nil {
		t.Error("abort overwrote a done run")
	}
}

// Aborting is the supported way to close a run, and it has to leave a record
// rather than only a status.
func TestAbortRecordsTheReasonInTheEventLog(t *testing.T) {
	root := t.TempDir()
	stoppedRun(t, root, "demo", state.StatusAwaitingHuman)

	if err := runAbort([]string{
		"--workspace", root, "--slug", "demo", "--reason", "superseded by another slug",
	}); err != nil {
		t.Fatal(err)
	}

	after := reload(t, root, "demo")
	if after.Status != state.StatusCancelled {
		t.Errorf("status = %s, want cancelled", after.Status)
	}

	events, err := state.ReadEvents(state.EventLogPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type == "run_aborted" && strings.Contains(string(event.Payload), "superseded by another slug") {
			found = true
		}
	}
	if !found {
		t.Errorf("no run_aborted event carrying the reason: %+v", events)
	}
}

func TestListFiltersByStatus(t *testing.T) {
	root := t.TempDir()
	stoppedRun(t, root, "stopped-one", state.StatusFailed)
	stoppedRun(t, root, "waiting-one", state.StatusAwaitingHuman)

	if err := runList([]string{"--workspace", root, "--status", "failed"}); err != nil {
		t.Fatal(err)
	}
	if err := runList([]string{"--workspace", root}); err != nil {
		t.Fatal(err)
	}
	// An unknown status is not an error: it is a question with no answer.
	if err := runList([]string{"--workspace", root, "--status", "nonsense"}); err != nil {
		t.Errorf("filtering on an unused status errored: %v", err)
	}
}

func TestIdleForUsesTheCoarsestUsefulUnit(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	for _, testCase := range []struct {
		ago  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{45 * time.Minute, "45m"},
		{5 * time.Hour, "5h"},
		{72 * time.Hour, "3d"},
	} {
		if got := idleFor(now, now.Add(-testCase.ago)); got != testCase.want {
			t.Errorf("idleFor(%s) = %q, want %q", testCase.ago, got, testCase.want)
		}
	}
}

// Raising the transition count on a run that ran out of wallclock leaves it
// stopping again on the next advance, having reported a resume that changed
// nothing.
func TestResumeRaisesTheBudgetThatStoppedTheRun(t *testing.T) {
	for _, testCase := range []struct {
		stoppedBy string
		check     func(*state.Run) int
		want      int
	}{
		{"tokens", func(r *state.Run) int { return r.TokenBudget }, 500},
		{"wallclock", func(r *state.Run) int { return r.WallclockSeconds }, 500},
		{"transitions", func(r *state.Run) int { return r.MaxTransitions }, 550},
	} {
		root := t.TempDir()
		run := stoppedRun(t, root, "demo", state.StatusBudgetExceeded)
		run.StoppedBy = testCase.stoppedBy
		if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
			t.Fatal(err)
		}

		if err := runResume([]string{
			"--workspace", root, "--slug", "demo", "--reason", "worth more", "--budget", "500",
		}); err != nil {
			t.Fatalf("%s: %v", testCase.stoppedBy, err)
		}
		if got := testCase.check(reload(t, root, "demo")); got != testCase.want {
			t.Errorf("%s: budget = %d, want %d", testCase.stoppedBy, got, testCase.want)
		}
	}
}
