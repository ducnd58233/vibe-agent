package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A reply naming the file, the path, or the bare slug is the same answer.
// Grading them differently would score the model on formatting and call it
// routing.
func TestAnAnswerIsGradedByTheAssetItNamesNotItsShape(t *testing.T) {
	for _, reply := range []string{
		"performance-optimization",
		"`performance-optimization`",
		"performance-optimization.md",
		"../skills/performance-optimization/SKILL.md",
		".ai-agents/skills/performance-optimization/SKILL.md",
		"  Performance-Optimization  ",
	} {
		if got := normalizeAnswer(reply); got != "performance-optimization" {
			t.Errorf("normalizeAnswer(%q) = %q", reply, got)
		}
	}
}

// The prompt asks for one bare line and the model sometimes explains anyway. An
// explanation that ends in the answer is the common shape, so the last non-empty
// line is the one graded.
func TestAnExplanationEndingInTheAnswerIsStillGraded(t *testing.T) {
	reply := "Looking at the routers, this is a latency question rather than a\n" +
		"query-plan one, so:\n\nperformance-optimization\n"
	if got := normalizeAnswer(reply); got != "performance-optimization" {
		t.Errorf("normalizeAnswer = %q, want the slug on the last line", got)
	}
}

func TestCodexJSONOutputIsGradedByFinalAgentMessage(t *testing.T) {
	reply := `{"type":"thread.started","thread_id":"1"}` + "\n" +
		`{"type":"item.completed","item":{"type":"agent_message","text":"performance-optimization"}}` + "\n" +
		`{"type":"turn.completed","usage":{"input_tokens":1}}`

	if got := normalizeAnswer(reply); got != "performance-optimization" {
		t.Errorf("normalizeAnswer = %q, want final Codex agent message", got)
	}
}

func TestAnEmptyAnswerIsNotAPass(t *testing.T) {
	if got := normalizeAnswer("   \n\n  "); got != "" {
		t.Errorf("normalizeAnswer of nothing = %q, want empty", got)
	}
}

func TestRunnerPresetsCanBeCombined(t *testing.T) {
	runners, err := resolveRunners([]string{"codex,claude", "cursor"})
	if err != nil {
		t.Fatalf("resolveRunners: %v", err)
	}
	if got := runnerNames(runners); got != "codex, claude, cursor" {
		t.Errorf("runnerNames = %q", got)
	}
	if runners[0].Command != "codex exec --ephemeral --sandbox read-only --json -" {
		t.Errorf("codex command = %q", runners[0].Command)
	}
	if runners[1].Command != "claude -p" {
		t.Errorf("claude command = %q", runners[1].Command)
	}
	if !runners[2].PromptAsArg {
		t.Error("cursor preset should pass the prompt as an argument")
	}
}

func TestAllRunnerPresetIncludesKnownHosts(t *testing.T) {
	runners, err := resolveRunners([]string{"all"})
	if err != nil {
		t.Fatalf("resolveRunners: %v", err)
	}
	if got := runnerNames(runners); got != "codex, claude, cursor, opencode, kimi, muse, antigravity" {
		t.Errorf("runnerNames = %q", got)
	}
}

func TestCustomRunnerFallsBackToStdinCommand(t *testing.T) {
	runners, err := resolveRunners([]string{"some-model -p"})
	if err != nil {
		t.Fatalf("resolveRunners: %v", err)
	}
	if len(runners) != 1 || runners[0].Name != "some-model -p" || runners[0].Command != "some-model -p" {
		t.Errorf("custom runner was not preserved: %+v", runners)
	}
	if runners[0].PromptAsArg {
		t.Error("custom runner should use stdin by default")
	}
}

// scripted replies to whichever fixture the prompt carries, one entry per call,
// so a script of three entries produces three different trials. Guarded because
// the runner asks fixtures in parallel.
type scripted struct {
	mu      sync.Mutex
	replies map[string][]string
	seen    map[string]int
	err     error
}

func newScripted(replies map[string][]string) *scripted {
	return &scripted{replies: replies, seen: map[string]int{}}
}

func (s *scripted) ask(_ context.Context, prompt string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	for intent, script := range s.replies {
		if strings.Contains(prompt, intent) {
			reply := script[s.seen[intent]%len(script)]
			s.seen[intent]++
			return reply, nil
		}
	}
	return "", fmt.Errorf("no scripted reply for that prompt")
}

func fixturesFor(intents ...string) []fixture {
	var out []fixture
	for i := 0; i+1 < len(intents); i += 2 {
		out = append(out, fixture{Intent: intents[i], Family: "skill", Slug: intents[i+1]})
	}
	return out
}

// pass^k and pass@k answer different questions, and the gap between them is the
// whole point: a fixture that routes correctly twice out of three times is not a
// wrong route, it is an ambiguous one, and the two need different fixes.
func TestPassingEveryTrialIsScoredApartFromPassingOnce(t *testing.T) {
	fixtures := fixturesFor(
		"steady intent", "always-right",
		"ambiguous intent", "sometimes-right",
		"broken intent", "never-right",
	)
	ask := newScripted(map[string][]string{
		"steady intent":    {"always-right", "always-right", "always-right"},
		"ambiguous intent": {"sometimes-right", "something-else", "sometimes-right"},
		"broken intent":    {"something-else", "something-else", "something-else"},
	})

	results := runRouting(t.Context(), fixtures, "ROUTERS", ask.ask, 3, 1)

	byIntent := map[string]outcome{}
	for _, result := range results {
		byIntent[result.Intent] = result
	}
	if got := byIntent["steady intent"].Passes; got != 3 {
		t.Errorf("a fixture answered correctly three times passed %d", got)
	}
	if got := byIntent["ambiguous intent"].Passes; got != 2 {
		t.Errorf("a fixture answered correctly twice passed %d", got)
	}
	if got := byIntent["broken intent"].Passes; got != 0 {
		t.Errorf("a fixture never answered correctly passed %d", got)
	}

	// The reported rate is pass^k, so only the steady fixture counts.
	if rate := reportRouting(results, 3); rate < 0.33 || rate > 0.34 {
		t.Errorf("pass^3 rate = %.2f, want one of three", rate)
	}
}

// A model that will not answer is a miss, not a crash. A nightly run that dies
// on the first rate limit reports nothing about the fixtures it never reached.
func TestAnAskerThatFailsCountsAsAMissRatherThanStoppingTheRun(t *testing.T) {
	ask := newScripted(nil)
	ask.err = fmt.Errorf("rate limited")

	results := runRouting(t.Context(), fixturesFor("some intent", "some-asset"), "ROUTERS", ask.ask, 2, 1)

	if len(results) != 1 {
		t.Fatalf("want one result, got %d", len(results))
	}
	if results[0].Passes != 0 {
		t.Errorf("a fixture that never got an answer passed %d times", results[0].Passes)
	}
	if len(results[0].Got) != 2 {
		t.Errorf("want an entry per trial even when each failed, got %v", results[0].Got)
	}
	for _, got := range results[0].Got {
		if !strings.HasPrefix(got, "error:") {
			t.Errorf("a failed trial recorded %q rather than the error", got)
		}
	}
}

func TestRunnerErrorWithNoOutputSaysSo(t *testing.T) {
	err := runnerError(nil, fmt.Errorf("exit status 1"), "", "")

	if got := err.Error(); got != "exit status 1 (no stderr/stdout)" {
		t.Errorf("runnerError = %q", got)
	}
}

func TestRunnerErrorKeepsOnlyUsefulTail(t *testing.T) {
	err := runnerError(nil, fmt.Errorf("exit status 1"),
		"noise 1\nnoise 2\nnoise 3\nnoise 4\nreal clue\nlast clue\n",
		"")

	got := err.Error()
	for _, want := range []string{"stderr:", "noise 3", "noise 4", "real clue", "last clue"} {
		if !strings.Contains(got, want) {
			t.Errorf("runnerError missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "noise 1") || strings.Contains(got, "noise 2") {
		t.Errorf("runnerError kept stale noise: %q", got)
	}
}

func TestRunnerTimeoutNamesTheTimeout(t *testing.T) {
	err := runnerError(context.DeadlineExceeded, fmt.Errorf("signal: killed"), "", "partial")

	if got := err.Error(); !strings.Contains(got, "runner timed out") || !strings.Contains(got, "stdout: partial") {
		t.Errorf("runnerError did not name the timeout and output: %q", got)
	}
}

// Every worker writes its own slot. Running the fixtures in parallel must not
// let one intent's answers land against another's expectation.
func TestParallelJobsKeepEachFixturesAnswers(t *testing.T) {
	fixtures := fixturesFor(
		"alpha intent", "alpha",
		"beta intent", "beta",
		"gamma intent", "gamma",
		"delta intent", "delta",
	)
	ask := newScripted(map[string][]string{
		"alpha intent": {"alpha"},
		"beta intent":  {"beta"},
		"gamma intent": {"gamma"},
		"delta intent": {"delta"},
	})

	results := runRouting(t.Context(), fixtures, "ROUTERS", ask.ask, 1, 4)

	for _, result := range results {
		if result.Passes != 1 {
			t.Errorf("%q wanted %s and got %v", result.Intent, result.Want, result.Got)
		}
	}
}

// The eval reads the same table doctor validates, so a row doctor accepts has to
// be a row the eval can run.
func TestFixturesComeFromTheSameTableDoctorChecks(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Latency regressed on an API path | skill | [`x`](../skills/performance-optimization/SKILL.md) |\n"+
			"| Run the pre-ship fan-out | command | [`ship.md`](../commands/ship.md) |\n",
		".ai-agents/skills/performance-optimization/SKILL.md",
		".ai-agents/commands/ship.md")

	fixtures, err := loadFixtures(root)
	if err != nil {
		t.Fatalf("loadFixtures: %v", err)
	}
	if len(fixtures) != 2 {
		t.Fatalf("want 2 fixtures, got %d: %+v", len(fixtures), fixtures)
	}
	if fixtures[0].Slug != "performance-optimization" {
		t.Errorf("skill slug = %q, want the directory name", fixtures[0].Slug)
	}
	if fixtures[1].Slug != "ship" {
		t.Errorf("command slug = %q, want the filename without .md", fixtures[1].Slug)
	}
}

func TestOnlyNarrowsTheRunToMatchingIntents(t *testing.T) {
	fixtures := fixturesFor("latency regressed", "a", "record a decision", "b")
	if kept := keepMatching(fixtures, "LATENCY"); len(kept) != 1 || kept[0].Slug != "a" {
		t.Errorf("--only did not narrow to the matching intent: %+v", kept)
	}
}

// The prompt has to carry the routers and the intent; without either the model
// is being asked to guess and the score means nothing.
func TestThePromptCarriesTheRoutersAndTheIntent(t *testing.T) {
	prompt := routingPrompt("### "+filepath.ToSlash(".ai-agents/ROUTER.md")+"\n\nTABLE", "record a decision")
	for _, want := range []string{"TABLE", "record a decision", ".ai-agents/ROUTER.md"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}
