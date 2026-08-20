package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/agent/infra/hostrunner"
	"github.com/ducnd58233/vibe-agent/runtime/internal/hosts"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/markdown"
)

// The routing fixtures name the asset each intent should reach. checkRoutingEvals
// proves that table is well-formed - links resolve, families match, no intent is
// claimed twice. None of that proves a model reading the routers actually lands
// on the named asset, which is the only thing the fixtures were written to
// promise. This runs that.
//
// It is deliberately not part of `doctor` and not part of CI. A model call is
// non-deterministic, needs credentials, and takes seconds, so gating a merge on
// one trades a stable build for a signal that moves slowly and reports noise as
// regression. The deterministic checks stay in the blocking path; this runs
// locally or nightly, and only fails when asked to with --require.
//
// What it measures is router legibility: given the router tables and nothing
// else, does the intent reach the asset. A live session carries far more context
// - the workspace rules, the loaded skill list, the conversation - so a pass
// here is necessary rather than sufficient. It is still the question the routers
// exist to answer, and it was going unasked.

// asker turns one prompt into one answer. It is injected so the eval can be
// tested without spending a model call, which is also what keeps the grading and
// the aggregation under test while the model stays out of the suite.
//
// The context is what makes a hung call survivable: one model invocation that
// never returns would otherwise hold a worker for the length of the run.
type asker func(ctx context.Context, prompt string) (string, error)

// fixture is one row of routing-evals.md reduced to what the eval needs.
type fixture struct {
	Intent string
	Family string
	Slug   string
}

// outcome is one fixture's result across every trial run for it.
type outcome struct {
	Fixture fixture  `json:"-"`
	Intent  string   `json:"intent"`
	Want    string   `json:"want"`
	Got     []string `json:"got"`
	Passes  int      `json:"passes"`
	Trials  int      `json:"trials"`
}

type runnerFlag []string

func (r *runnerFlag) String() string {
	return strings.Join(*r, ",")
}

func (r *runnerFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*r = append(*r, part)
		}
	}
	return nil
}

type runnerSpec struct {
	Name        string
	Command     string
	PromptAsArg bool
}

func runnerFromHost(host hosts.Host, evalName string) runnerSpec {
	return runnerSpec{
		Name:        evalName,
		Command:     host.EvalCommand,
		PromptAsArg: host.PromptAsArg,
	}
}

func resolveRunner(value string) ([]runnerSpec, error) {
	if value == "all" {
		var out []runnerSpec
		for _, name := range hosts.EvalRunnerNames() {
			host, ok := hosts.EvalHost(name)
			if !ok {
				return nil, fmt.Errorf("unknown eval runner %q", name)
			}
			out = append(out, runnerFromHost(host, name))
		}
		return out, nil
	}
	if host, ok := hosts.EvalHost(value); ok {
		return []runnerSpec{runnerFromHost(host, value)}, nil
	}
	return []runnerSpec{{Name: value, Command: value}}, nil
}

// routerFiles are what a session reads when deciding where to go: the hub, then
// the folder tables it points at.
var routerFiles = []string{
	filepath.Join(".ai-agents", "ROUTER.md"),
	filepath.Join(".ai-agents", "skills", "ROUTER.md"),
	filepath.Join(".ai-agents", "agents", "ROUTER.md"),
	filepath.Join(".ai-agents", "commands", "ROUTER.md"),
}

func evalCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("eval needs a subcommand: routing or graph")
	}
	switch args[0] {
	case "routing":
		return routingEvalCommand(args[1:])
	case "graph":
		return graphEvalCommand(args[1:])
	default:
		return fmt.Errorf("unknown eval %q; try routing or graph", args[0])
	}
}

func routingEvalCommand(args []string) error {
	flags := newFlagSet("eval routing")
	paths := addRootFlags(flags)
	trials := flags.Int("trials", 1, "how many times to ask each intent")
	jobs := flags.Int("jobs", 4, "fixtures asked in parallel")
	var runnerArgs runnerFlag
	flags.Var(&runnerArgs, "runner",
		"runner preset or command; repeat or comma-separate. Presets: codex, claude, cursor, opencode, all")
	only := flags.String("only", "", "run fixtures whose intent contains this text")
	timeout := flags.Duration("timeout", 2*time.Minute, "how long one model call may take")
	asJSON := flags.String("json", "", "also write the full result to this file")
	require := flags.Float64("require", 0,
		"exit non-zero when the pass-every-trial rate is below this (0 to 1)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	_, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}
	if *trials < 1 || *jobs < 1 {
		return fmt.Errorf("--trials and --jobs must be at least 1")
	}
	runners, err := resolveRunners(runnerArgs)
	if err != nil {
		return err
	}

	fixtures, err := loadFixtures(toolkitRoot)
	if err != nil {
		return err
	}
	if *only != "" {
		fixtures = keepMatching(fixtures, *only)
	}
	if len(fixtures) == 0 {
		return fmt.Errorf("no fixtures to run")
	}

	routers, err := routerContext(toolkitRoot)
	if err != nil {
		return err
	}

	fmt.Printf("routing eval  %d fixtures x %d trial(s)  runner(s): %s\n\n",
		len(fixtures), *trials, runnerNames(runners))

	allResults := map[string][]outcome{}
	failing := []string{}
	for i, runner := range runners {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("runner %s  command: %s\n\n", runner.Name, runner.Command)
		results := runRouting(context.Background(), fixtures, routers,
			commandAsker(runner, *timeout), *trials, *jobs)
		rate := reportRouting(results, *trials)
		allResults[runner.Name] = results
		if *require > 0 && rate < *require {
			failing = append(failing, fmt.Sprintf("%s %.2f", runner.Name, rate))
		}
	}

	if *asJSON != "" {
		var payload any = allResults
		if len(runners) == 1 {
			payload = allResults[runners[0].Name]
		}
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Clean(*asJSON), encoded, 0o600); err != nil {
			return err
		}
		fmt.Printf("\nwrote %s\n", *asJSON)
	}

	if len(failing) > 0 {
		return fmt.Errorf("pass-every-trial rate below %.2f: %s", *require, strings.Join(failing, ", "))
	}
	return nil
}

func resolveRunners(values []string) ([]runnerSpec, error) {
	if len(values) == 0 {
		values = []string{"codex"}
	}
	var out []runnerSpec
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			resolved, err := resolveRunner(part)
			if err != nil {
				return nil, err
			}
			out = append(out, resolved...)
		}
	}
	return out, nil
}

func runnerNames(runners []runnerSpec) string {
	var names []string
	for _, runner := range runners {
		names = append(names, runner.Name)
	}
	return strings.Join(names, ", ")
}

// loadFixtures reads the rows the eval can run.
//
// Malformed rows are skipped rather than reported, because doctor already fails
// on them and saying it twice in two voices helps nobody.
func loadFixtures(toolkitRoot string) ([]fixture, error) {
	path := filepath.Join(toolkitRoot, ".ai-agents", "references", "routing-evals.md")
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read fixtures: %w", err)
	}
	var out []fixture
	for _, row := range markdown.ParseFirstTable(string(raw)) {
		if len(row.Cells) != 3 || row.Cells[0] == "" {
			continue
		}
		targets := markdown.LinkTargets(row.Cells[2])
		if len(targets) != 1 {
			continue
		}
		out = append(out, fixture{
			Intent: row.Cells[0],
			Family: row.Cells[1],
			Slug:   strings.ToLower(markdown.AssetSlug(targets[0])),
		})
	}
	return out, nil
}

func keepMatching(fixtures []fixture, text string) []fixture {
	needle := strings.ToLower(text)
	var kept []fixture
	for _, f := range fixtures {
		if strings.Contains(strings.ToLower(f.Intent), needle) {
			kept = append(kept, f)
		}
	}
	return kept
}

// routerContext is the material a session reads before choosing an asset.
func routerContext(toolkitRoot string) (string, error) {
	var parts []string
	for _, rel := range routerFiles {
		raw, err := os.ReadFile(filepath.Clean(filepath.Join(toolkitRoot, rel)))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", rel, err)
		}
		parts = append(parts, "### "+filepath.ToSlash(rel)+"\n\n"+string(raw))
	}
	return strings.Join(parts, "\n\n"), nil
}

// routingPrompt asks for the one thing the grader can check.
//
// The answer format is constrained to a bare slug because anything looser makes
// the grader guess, and a grader that guesses is measuring itself.
func routingPrompt(routers, intent string) string {
	return "You are choosing which toolkit asset handles a request. " +
		"Use only the router tables below.\n\n" +
		"<routers>\n" + routers + "\n</routers>\n\n" +
		"Request: " + intent + "\n\n" +
		"Answer with exactly one line: the asset's name with no path and no .md " +
		"extension (for a skill, its directory name). No explanation, no quotes."
}

// normalizeAnswer reduces a reply to the slug it names.
//
// The model is asked for a bare slug and sometimes explains anyway, so the last
// non-empty line is taken: an explanation that ends in the answer is common, one
// that begins with it is not. Path and extension are stripped so naming the file
// counts the same as naming the asset.
func normalizeAnswer(raw string) string {
	if final := finalJSONAgentMessage(raw); final != "" {
		raw = final
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	last := ""
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			last = trimmed
		}
	}
	last = strings.Trim(last, "`'\"*.,: ")
	if last == "" {
		return ""
	}
	return strings.ToLower(markdown.AssetSlug(filepath.ToSlash(last)))
}

func finalJSONAgentMessage(raw string) string {
	var last string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			last = event.Item.Text
		}
	}
	return last
}

// commandAsker runs the configured command, feeding the prompt on stdin.
//
// stdin rather than an argument because the router tables run to some fifteen
// kilobytes, which is comfortable on a pipe and close to the command-line limit
// on Windows.
func commandAsker(runner runnerSpec, timeout time.Duration) asker {
	return func(ctx context.Context, prompt string) (string, error) {
		spec, err := hostrunner.FromCommand(runner.Command, runner.PromptAsArg)
		if err != nil {
			return "", fmt.Errorf("empty --runner")
		}
		// The deadline stays here rather than inside the runner because
		// runnerError reads ctx.Err() to say "runner timed out" instead of
		// reporting the raw exit status, and that reading needs the context
		// this function owns.
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		response, runErr := hostrunner.New(spec).Run(ctx, agent.Request{Prompt: prompt})
		if runErr != nil {
			return "", runnerError(ctx.Err(), runErr, response.Stderr, response.Text)
		}
		return response.Text, nil
	}
}

const (
	runnerErrorLines = 4
	runnerErrorChars = 500
)

func runnerError(ctxErr, runErr error, stderr, stdout string) error {
	label := runErr.Error()
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		label = "runner timed out"
	}

	var parts []string
	if detail := compactRunnerOutput(stderr); detail != "" {
		parts = append(parts, "stderr: "+detail)
	}
	if detail := compactRunnerOutput(stdout); detail != "" {
		parts = append(parts, "stdout: "+detail)
	}
	if len(parts) == 0 {
		return fmt.Errorf("%s (no stderr/stdout)", label)
	}
	return fmt.Errorf("%s: %s", label, strings.Join(parts, "; "))
}

func compactRunnerOutput(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	var kept []string
	for i := len(lines) - 1; i >= 0 && len(kept) < runnerErrorLines; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			kept = append(kept, line)
		}
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	text := strings.Join(kept, " | ")
	if len(text) <= runnerErrorChars {
		return text
	}
	return "..." + text[len(text)-runnerErrorChars:]
}

// runRouting asks every fixture, trials times, jobs at a time.
//
// One worker owns a whole fixture rather than a single trial, so each writes its
// own slot and the trials of one intent stay together in time - which matters
// when a rate limit slows the run and the answers start to differ.
func runRouting(ctx context.Context, fixtures []fixture, routers string, ask asker, trials, jobs int) []outcome {
	results := make([]outcome, len(fixtures))
	queue := make(chan int)

	var wg sync.WaitGroup
	for range jobs {
		wg.Go(func() {
			for index := range queue {
				results[index] = askFixture(ctx, fixtures[index], routers, ask, trials)
			}
		})
	}
	for index := range fixtures {
		queue <- index
	}
	close(queue)
	wg.Wait()
	return results
}

func askFixture(ctx context.Context, f fixture, routers string, ask asker, trials int) outcome {
	result := outcome{Fixture: f, Intent: f.Intent, Want: f.Slug, Trials: trials}
	for range trials {
		answer, err := ask(ctx, routingPrompt(routers, f.Intent))
		if err != nil {
			result.Got = append(result.Got, "error: "+err.Error())
			continue
		}
		got := normalizeAnswer(answer)
		result.Got = append(result.Got, got)
		if got == f.Slug {
			result.Passes++
		}
	}
	return result
}

// reportRouting prints the two rates worth knowing and every fixture that did
// not pass every time, returning the pass-every-trial rate.
//
// pass^k - passing every trial - is the number that matters for a control plane,
// where "usually routes correctly" and "does not route correctly" are close to
// the same thing. pass@k is printed beside it because the gap between them is
// the flakiness, and a fixture that is flaky is telling you the router is
// ambiguous rather than wrong.
func reportRouting(results []outcome, trials int) float64 {
	always, ever := 0, 0
	for _, result := range results {
		if result.Passes == trials {
			always++
		}
		if result.Passes > 0 {
			ever++
		}
	}
	total := len(results)
	fmt.Printf("  pass^%d  %d/%d  %.0f%%\n", trials, always, total, percent(always, total))
	fmt.Printf("  pass@%d  %d/%d  %.0f%%\n", trials, ever, total, percent(ever, total))

	var never, flaky []outcome
	for _, result := range results {
		switch {
		case result.Passes == 0:
			never = append(never, result)
		case result.Passes < trials:
			flaky = append(flaky, result)
		}
	}
	printGroup("never routed correctly", never)
	printGroup("routed correctly only sometimes", flaky)

	return percent(always, total) / 100
}

func printGroup(label string, group []outcome) {
	if len(group) == 0 {
		return
	}
	fmt.Printf("\n  %s (%d):\n", label, len(group))
	for _, result := range group {
		fmt.Printf("    %q\n", result.Intent)
		fmt.Printf("      want %s, got %s\n", result.Want, tally(result.Got))
	}
}

// tally collapses repeated answers so three identical replies read as one line.
func tally(answers []string) string {
	counts := map[string]int{}
	for _, answer := range answers {
		counts[answer]++
	}
	var names []string
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	var parts []string
	for _, name := range names {
		if name == "" {
			name = "(no answer)"
		}
		parts = append(parts, fmt.Sprintf("%s x%d", name, counts[name]))
	}
	return strings.Join(parts, ", ")
}

func percent(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}
