package main

import (
	"fmt"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/app"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/persistence"
)

// doctorCommand checks that the control plane is wired correctly in this
// workspace. Every check prints its own verdict, so a silent skip is visible.
func doctorCommand(args []string) error {
	flags := newFlagSet("doctor")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, toolkitRoot, err := paths.resolve()
	if err != nil {
		return err
	}

	report := &diagnostics{}

	fmt.Printf("vibe-agent %s\n", version)
	fmt.Printf("  workspace %s\n", workspaceRoot)
	fmt.Printf("  toolkit   %s\n\n", toolkitRoot)

	checkGraphs(report, toolkitRoot)
	checkGraphPathEvals(report, toolkitRoot)
	checkRoutingEvals(report, toolkitRoot)
	// The workspace, not the toolkit. A host reads hooks from the project
	// directory it was opened on, so a vendored toolkit's own settings.json is
	// wiring for opening the toolkit itself and reaches nothing here.
	checkHookWiring(report, workspaceRoot)
	checkCheckPlan(report, workspaceRoot, toolkitRoot)
	checkHumanVerifiers(workspaceRoot)
	checkTaskFiles(report, workspaceRoot)
	checkAutoOptIn(report, workspaceRoot)
	checkMemory(report, workspaceRoot)
	checkRunState(report, workspaceRoot)
	checkWebState(report, workspaceRoot)
	checkGitignore(report, workspaceRoot)

	fmt.Println()
	if report.problems > 0 {
		return fmt.Errorf("doctor found %d problems", report.problems)
	}
	fmt.Println("doctor: OK")
	return nil
}

type diagnostics struct {
	problems int
}

func (d *diagnostics) check(label string, ok bool, detail string) {
	if ok {
		fmt.Printf("  ok    %s\n", label)
		return
	}
	d.problems++
	fmt.Printf("  FAIL  %s: %s\n", label, detail)
}

func checkGraphs(report *diagnostics, toolkitRoot string) {
	graphs, err := graph.LoadDir(graph.DefaultDir(toolkitRoot))
	if err != nil {
		report.check("workflow graphs load and validate", false, err.Error())
		return
	}
	report.check(fmt.Sprintf("workflow graphs load and validate (%d)", len(graphs)), true, "")
}

// checkCheckPlan reports whether the workspace declares how its checks are
// produced, and whether every verifier node in every graph is covered.
//
// A missing plan is a warning rather than a failure: a workspace that has not
// started a run does not need one yet. A plan that misses a check a graph will
// reach is a failure, because the run would stall at that node with no way past
// it, and finding that mid-delivery is the expensive time to find it.
func checkCheckPlan(report *diagnostics, workspaceRoot, toolkitRoot string) {
	path := checkplan.DefaultPath(workspaceRoot)
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("  note  no %s yet; verifier nodes need one before a run reaches them\n", checkplan.FileName)
		return
	}

	plan, err := checkplan.Load(path)
	if err != nil {
		report.check("check plan loads and validates", false, err.Error())
		return
	}
	report.check(fmt.Sprintf("check plan loads and validates (%d checks)", len(plan.Names())), true, "")

	graphs, err := graph.LoadDir(graph.DefaultDir(toolkitRoot))
	if err != nil {
		return // already reported by checkGraphs
	}
	for id, loaded := range graphs {
		var missing, optional []string
		for name, node := range loaded.Spec.Nodes {
			if node.Type != graph.NodeVerifier || node.Check == "" {
				continue
			}
			if _, err := plan.Entry(node.Check); err == nil {
				continue
			}
			// An undeclared check is only a defect where the graph does not
			// already treat "not declared here" as an answer.
			//
			// A guard with acceptsSkipped says the opposite: omitting the check
			// is the tracked statement that this workspace has none, and the
			// runner routes past it as skipped rather than passed. doctor used
			// to fail those anyway, so it contradicted the runner about e2e -
			// the graph documents omitting it as supported, the runtime skips
			// it visibly, and this said the workspace was broken.
			//
			// The contradiction only became visible when a second such node was
			// added. Every workspace shares this graph and keeps its own check
			// plan, so without this a new verifier node breaks doctor in every
			// consumer repository on the day it lands.
			if acceptsSkipped(loaded, node.Check) {
				optional = append(optional, fmt.Sprintf("%s (node %s)", node.Check, name))
				continue
			}
			missing = append(missing, fmt.Sprintf("%s (node %s)", node.Check, name))
		}
		sort.Strings(missing)
		report.check("graph "+id+": every verifier node has a planned check",
			len(missing) == 0,
			"undeclared in "+checkplan.FileName+": "+strings.Join(missing, ", "))

		if len(optional) > 0 {
			sort.Strings(optional)
			fmt.Printf("  note  not declared in %s, so the run will skip it visibly: %s\n",
				checkplan.FileName, strings.Join(optional, ", "))
		}
	}
}

// acceptsSkipped reports whether any guard reading this check treats a skip as
// an answer.
func acceptsSkipped(loaded *graph.Graph, check string) bool {
	for _, guard := range loaded.Spec.Guards {
		if guard.Key() == check && guard.AcceptsSkipped {
			return true
		}
	}
	return false
}

// checkMemory reports on the store without creating one.
//
// memory.Open creates, so this used to leave a .agent-state/ behind in whatever
// directory doctor was run from, including directories with nothing to do with
// the toolkit. The memory package states the rule it was breaking: reads never
// create state, which is why recall opens the database only if it exists.
// checkRepoMap beside it follows the same rule.
func checkMemory(report *diagnostics, workspaceRoot string) {
	store, exists, err := openExistingMemory(workspaceRoot)
	if err != nil {
		report.check("memory database opens", false, err.Error())
		return
	}
	if !exists {
		fmt.Println("  note  no memory database yet; one is created the first time " +
			"something is stored")
		return
	}
	report.check("memory database opens", true, "")
	_ = store.Close()
}

// checkRunState loads every manifest under tmp/, so a run that would fail to
// resume is found now rather than mid-delivery.
func checkRunState(report *diagnostics, workspaceRoot string) {
	slugs, err := state.List(workspaceRoot)
	if err != nil {
		return
	}
	now := time.Now()
	var stale []string
	for _, slug := range slugs {
		path := state.ManifestPath(workspaceRoot, slug)
		current, err := state.Load(path)
		if err != nil {
			report.check("run state "+slug+" is valid", false, err.Error())
			continue
		}
		report.check("run state "+slug+" is valid", true, "")
		if idle := idleRun(current, now); idle != "" {
			stale = append(stale, idle)
		}
	}

	// A note rather than a check, because an idle run is not a broken one and
	// doctor failing on it would teach people to stop reading doctor.
	//
	// It exists because sixteen of these accumulated before anyone counted
	// them: a parked run and an active one look identical in `run list`, so
	// nothing said the backlog was growing until someone read the whole list.
	for _, line := range stale {
		fmt.Println("  note  " + line)
	}
	if len(stale) > 0 {
		fmt.Printf("  note  %d run(s) idle past %d days; close one with `run abort --slug <slug> --reason <why>`\n",
			len(stale), int(idleThreshold.Hours()/24))
	}
}

// idleThreshold is how long a run may sit before doctor mentions it.
//
// Three days rather than one: a run picked up again after a weekend is not a
// backlog, and a threshold that fires on those gets ignored. Rather than a
// setting, because the number only has to be roughly right, and a knob here
// would be one more thing to tune instead of one more run to close.
const idleThreshold = 72 * time.Hour

// idleRun reports a run that has sat too long, or "" for one that has not.
//
// A run that finished is finished, and saying so every time doctor runs would
// bury the ones actually waiting. So done, failed, and cancelled are exempt.
//
// budget_exceeded is deliberately not, even though the runner treats it as
// terminal. It is the one stop a person can undo - `run resume` raises the
// budget and carries on - so it is a run waiting for a decision rather than a
// run that reached one, which is exactly the shape this note exists to find.
func idleRun(current *state.Run, now time.Time) string {
	switch current.Status {
	case state.StatusDone, state.StatusFailed, state.StatusCancelled:
		return ""
	}
	if current.UpdatedAt.IsZero() || now.Sub(current.UpdatedAt) < idleThreshold {
		return ""
	}
	return fmt.Sprintf("run %s is %s idle at %s (%s)",
		current.Slug, idleFor(now, current.UpdatedAt), current.CurrentNode, current.Status)
}

func checkWebState(report *diagnostics, workspaceRoot string) {
	st, exists, err := persistence.LoadState(workspaceRoot)
	if err != nil {
		report.check("web.json is valid", false, err.Error())
		return
	}
	if !exists {
		fmt.Println("  note  no web.json yet; start `vibe-agent web` when you want the UI")
		return
	}
	parsed, parseErr := url.Parse(st.URL)
	if parseErr != nil {
		report.check("web.json URL is loopback", false, parseErr.Error())
		return
	}
	if err := app.ValidateListenHost(parsed.Hostname()); err != nil {
		report.check("web.json URL is loopback", false, st.URL)
		return
	}
	report.check("web.json URL is loopback", true, "")
}

// checkGitignore catches the mistake that would commit run evidence or a memory
// database, both of which are workspace-local state rather than assets.
func checkGitignore(report *diagnostics, workspaceRoot string) {
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(workspaceRoot, ".gitignore")))
	if err != nil {
		return // no .gitignore is a consumer choice, not a runtime problem
	}
	content := string(raw)
	report.check("tmp/ is gitignored", strings.Contains(content, "/tmp/"),
		"add /tmp/ so run evidence is not committed")
	report.check(workspace.StateDirName+"/ is gitignored",
		strings.Contains(content, "/"+workspace.StateDirName+"/"),
		"add /"+workspace.StateDirName+"/ so derived state is not committed")
}
