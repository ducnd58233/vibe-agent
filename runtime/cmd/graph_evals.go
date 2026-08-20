package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// Graph path evals answer a question the routing evals structurally cannot:
// given these check results, which nodes does a run actually visit?
//
// Routing evals grade a model's choice of asset. This grades the graph itself,
// with no model in the loop, so it runs in CI on every change. That matters most
// for edges added later: an edge that bypasses a gate is one line of YAML, and
// nothing else in the suite would notice it started firing on the default path.
//
// The fixtures sit in references/ beside routing-evals.md rather than in
// graphs/, because the graph loader treats every YAML file in that directory as
// a WorkflowGraph and would reject a fixture file as a malformed graph.

// PathEvalsFileName is the fixture file, read from the references directory.
const PathEvalsFileName = "graph-path-evals.yaml"

const (
	pathEvalsAPIVersion = "vibe-agent/v1"
	pathEvalsKind       = "GraphPathEvals"
)

type pathEvalFile struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Description string `yaml:"description"`
	} `yaml:"metadata"`
	Spec struct {
		Scenarios []pathScenario `yaml:"scenarios"`
	} `yaml:"spec"`
}

// pathScenario is one walk: a graph, the outcomes each step reports, and the
// node sequence that must result.
type pathScenario struct {
	Name  string          `yaml:"name"`
	Graph string          `yaml:"graph"`
	Flags map[string]bool `yaml:"flags"`
	Steps []pathStep      `yaml:"steps"`
	// Expect.Path starts with the initial node and adds one entry per step, so
	// a step that leaves the run where it was repeats the node rather than
	// hiding it. Reading the list is reading the run's position over time.
	Expect pathExpect `yaml:"expect"`
}

type pathStep struct {
	Check   *pathCheck      `yaml:"check"`
	Result  map[string]bool `yaml:"result"`
	Blocker string          `yaml:"blocker"`
}

// pathCheck names its source for the same reason run state does: evidence
// without provenance is the thing this control plane refuses to record. A
// fixture that omitted it would be describing a state the runtime cannot reach.
type pathCheck struct {
	Name    string `yaml:"name"`
	Passed  bool   `yaml:"passed"`
	Skipped bool   `yaml:"skipped"`
	Source  string `yaml:"source"`
}

type pathExpect struct {
	Path   []string `yaml:"path"`
	Status string   `yaml:"status"`
}

// parsePathEvals reads and shape-checks the fixture file.
func parsePathEvals(raw []byte) (*pathEvalFile, error) {
	var file pathEvalFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse path evals: %w", err)
	}
	if file.APIVersion != pathEvalsAPIVersion {
		return nil, fmt.Errorf("path evals: apiVersion %q, want %q", file.APIVersion, pathEvalsAPIVersion)
	}
	if file.Kind != pathEvalsKind {
		return nil, fmt.Errorf("path evals: kind %q, want %q", file.Kind, pathEvalsKind)
	}
	seen := map[string]bool{}
	for i, scenario := range file.Spec.Scenarios {
		switch {
		case scenario.Name == "":
			return nil, fmt.Errorf("path evals: scenario %d has no name", i)
		case scenario.Graph == "":
			return nil, fmt.Errorf("path evals: scenario %q names no graph", scenario.Name)
		case len(scenario.Expect.Path) == 0:
			return nil, fmt.Errorf("path evals: scenario %q expects no path", scenario.Name)
		case seen[scenario.Name]:
			return nil, fmt.Errorf("path evals: scenario %q is declared twice", scenario.Name)
		}
		seen[scenario.Name] = true
		for j, step := range scenario.Steps {
			if step.Check == nil {
				continue
			}
			if step.Check.Name == "" {
				return nil, fmt.Errorf("path evals: scenario %q step %d has a check with no name", scenario.Name, j)
			}
			if step.Check.Source == "" {
				return nil, fmt.Errorf("path evals: scenario %q step %d records %q with no source; evidence needs provenance",
					scenario.Name, j, step.Check.Name)
			}
		}
	}
	return &file, nil
}

// pathEvalsPath is where the fixtures live for a toolkit root.
func pathEvalsPath(toolkitRoot string) string {
	return filepath.Join(toolkitRoot, ".ai-agents", "references", PathEvalsFileName)
}

// loadPathEvals reads the fixture file. A missing file is not an error: a
// consumer repo need not ship path fixtures.
func loadPathEvals(toolkitRoot string) (*pathEvalFile, bool, error) {
	raw, err := os.ReadFile(filepath.Clean(pathEvalsPath(toolkitRoot)))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	file, err := parsePathEvals(raw)
	if err != nil {
		return nil, true, err
	}
	return file, true, nil
}

// walkPathScenario runs one scenario and reports the nodes visited.
//
// The clock is fixed so a scenario is a pure function of its fixture. Nothing
// here reads or writes run state on disk: the walk is in memory, which is what
// keeps it runnable in CI with no workspace.
func walkPathScenario(graphDir string, scenario pathScenario) (path []string, status string, err error) {
	loaded, err := graph.LoadByID(graphDir, scenario.Graph)
	if err != nil {
		return nil, "", fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}

	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	run, err := state.NewRun("path-eval", "graph path eval", loaded.Metadata.ID, loaded.Spec.MaxTransitions, at)
	if err != nil {
		return nil, "", fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}

	runner := loop.New(loaded)
	runner.Now = func() time.Time { return at }
	if err := runner.Enter(run); err != nil {
		return nil, "", fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	if run.Flags == nil {
		run.Flags = map[string]bool{}
	}
	for name, value := range scenario.Flags {
		run.Flags[name] = value
	}

	path = append(path, run.CurrentNode)
	for i, step := range scenario.Steps {
		outcome := loop.Outcome{Result: step.Result, Blocker: step.Blocker}
		if step.Check != nil {
			outcome.Check = &loop.NamedCheck{
				Name: step.Check.Name,
				Check: state.Check{
					Passed:  step.Check.Passed,
					Skipped: step.Check.Skipped,
					Source:  state.CheckSource(step.Check.Source),
					At:      at,
				},
			}
		}
		transition, err := runner.Advance(run, outcome)
		if err != nil {
			return path, string(run.Status), fmt.Errorf("scenario %q step %d: %w", scenario.Name, i, err)
		}
		path = append(path, run.CurrentNode)
		if transition.Terminal {
			break
		}
	}
	return path, string(run.Status), nil
}

// runPathScenario walks a scenario and compares the result to what it expects.
func runPathScenario(graphDir string, scenario pathScenario) error {
	path, status, err := walkPathScenario(graphDir, scenario)
	if err != nil {
		return err
	}
	if strings.Join(path, " -> ") != strings.Join(scenario.Expect.Path, " -> ") {
		return fmt.Errorf("scenario %q walked %s, expected %s",
			scenario.Name, strings.Join(path, " -> "), strings.Join(scenario.Expect.Path, " -> "))
	}
	if scenario.Expect.Status != "" && status != scenario.Expect.Status {
		return fmt.Errorf("scenario %q ended %s, expected %s", scenario.Name, status, scenario.Expect.Status)
	}
	return nil
}

// pathEvalCoverage reports how many of a graph's nodes at least one scenario
// visits. Nodes no scenario reaches are the ones a change can break unnoticed.
func pathEvalCoverage(graphDir string, file *pathEvalFile) (covered, total int, uncovered []string, err error) {
	graphs, err := graph.LoadDir(graphDir)
	if err != nil {
		return 0, 0, nil, err
	}

	visited := map[string]map[string]bool{}
	for _, scenario := range file.Spec.Scenarios {
		path, _, walkErr := walkPathScenario(graphDir, scenario)
		if walkErr != nil {
			continue
		}
		if visited[scenario.Graph] == nil {
			visited[scenario.Graph] = map[string]bool{}
		}
		for _, node := range path {
			visited[scenario.Graph][node] = true
		}
	}

	for id, loaded := range graphs {
		for node := range loaded.Spec.Nodes {
			total++
			if visited[id][node] {
				covered++
				continue
			}
			uncovered = append(uncovered, id+"/"+node)
		}
	}
	sort.Strings(uncovered)
	return covered, total, uncovered, nil
}

// checkGraphPathEvals is the doctor half: parse, run, and report coverage.
func checkGraphPathEvals(report *diagnostics, toolkitRoot string) {
	graphDir := graph.DefaultDir(toolkitRoot)
	file, present, err := loadPathEvals(toolkitRoot)
	if err != nil {
		report.check("graph path evals parse", false, err.Error())
		return
	}
	if !present {
		fmt.Printf("  note  no %s in this toolkit; graph paths are unexercised\n", PathEvalsFileName)
		return
	}
	report.check(fmt.Sprintf("graph path evals parse (%d scenarios)", len(file.Spec.Scenarios)), true, "")

	failures := []string{}
	for _, scenario := range file.Spec.Scenarios {
		if err := runPathScenario(graphDir, scenario); err != nil {
			failures = append(failures, err.Error())
		}
	}
	report.check("graph path evals: every scenario walks its expected path",
		len(failures) == 0, strings.Join(failures, "; "))

	covered, total, uncovered, err := pathEvalCoverage(graphDir, file)
	if err != nil {
		report.check("graph path eval coverage", false, err.Error())
		return
	}
	fmt.Printf("  note  graph path eval coverage %d/%d nodes", covered, total)
	if len(uncovered) > 0 {
		fmt.Printf(" (uncovered: %s)", strings.Join(uncovered, ", "))
	}
	fmt.Println()
}

// graphEvalCommand runs the scenarios from the CLI.
func graphEvalCommand(args []string) error {
	flags := newFlagSet("eval graph")
	paths := addRootFlags(flags)
	only := flags.String("only", "", "run scenarios whose name contains this text")
	if err := flags.Parse(args); err != nil {
		return err
	}
	_, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}

	graphDir := graph.DefaultDir(toolkitRoot)
	file, present, err := loadPathEvals(toolkitRoot)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("no %s in %s", PathEvalsFileName, filepath.Dir(pathEvalsPath(toolkitRoot)))
	}

	ran, failed := 0, 0
	for _, scenario := range file.Spec.Scenarios {
		if *only != "" && !strings.Contains(scenario.Name, *only) {
			continue
		}
		ran++
		if err := runPathScenario(graphDir, scenario); err != nil {
			failed++
			fmt.Printf("  FAIL  %s\n        %v\n", scenario.Name, err)
			continue
		}
		fmt.Printf("  ok    %s\n", scenario.Name)
	}

	covered, total, uncovered, err := pathEvalCoverage(graphDir, file)
	if err != nil {
		return err
	}
	fmt.Printf("\n%d scenarios, %d failed. Node coverage %d/%d.\n", ran, failed, covered, total)
	if len(uncovered) > 0 {
		fmt.Printf("Uncovered: %s\n", strings.Join(uncovered, ", "))
	}
	if failed > 0 {
		return fmt.Errorf("%d scenario(s) failed", failed)
	}
	return nil
}
