package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/autoconfig"
	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/sandbox"
	"github.com/ducnd58233/vibe-agent/runtime/internal/tasks"
)

// taskHeading matches the way a TASKS.md names one task, in either of the two
// shapes this toolkit's plans have used.
var taskHeading = regexp.MustCompile(`(?m)^#{2,3}\s+(Task\s+\d+|T\d+)\b`)

// checkHumanVerifiers reports how many checks a workspace answers with a
// person's word.
//
// The count is worth seeing rather than burying. Every one of them is a place
// where evidence has no machine behind it, and a plan that accumulates them has
// an honesty problem rather than a configuration problem.
func checkHumanVerifiers(workspaceRoot string) {
	plan, err := checkplan.Load(checkplan.DefaultPath(workspaceRoot))
	if err != nil {
		return // already reported by checkCheckPlan
	}
	var human []string
	for _, name := range plan.Names() {
		entry, entryErr := plan.Entry(name)
		if entryErr == nil && entry.Verifier == checkplan.HumanVerifier {
			human = append(human, name)
		}
	}
	if len(human) == 0 {
		fmt.Println("  note  no check is answered by a person alone")
		return
	}
	fmt.Printf("  note  %d check(s) answered by a person: %s\n", len(human), strings.Join(human, ", "))
}

// checkTaskFiles validates every task list in the workspace and refuses a done
// status whose TASKS.md acceptance checkboxes are still open.
//
// Count mismatch between prose headings and JSON tasks stays a note: TASKS.md
// may carry extra context. Incomplete acceptance criteria is a failure, because
// that is how a run was ending while work boxes remained unticked.
func checkTaskFiles(report *diagnostics, workspaceRoot string) {
	slugs, err := state.List(workspaceRoot)
	if err != nil {
		report.check("task lists are readable", false, err.Error())
		return
	}

	checked := 0
	for _, slug := range slugs {
		path := tasks.Path(workspaceRoot, slug)
		if _, statErr := os.Stat(path); statErr != nil {
			continue
		}
		file, loadErr := tasks.Load(workspaceRoot, slug)
		if loadErr != nil {
			report.check("task list "+slug+" loads and validates", false, loadErr.Error())
			continue
		}
		prosePath := tasks.ProsePath(workspaceRoot, slug)
		prose := ""
		if prosePath == "" {
			fmt.Printf("  note  %s has no TASKS prose beside its task list\n", slug)
		} else {
			raw, readErr := os.ReadFile(filepath.Clean(prosePath))
			if readErr != nil {
				report.check("task list "+slug+" TASKS prose is readable", false, readErr.Error())
				continue
			}
			prose = string(raw)
			if headings := len(taskHeading.FindAllString(prose, -1)); headings != len(file.Tasks) {
				fmt.Printf("  note  %s: TASKS prose names %d task(s), %s holds %d; the prose file may carry context the list does not\n",
					slug, headings, filepath.Base(path), len(file.Tasks))
			}
		}

		remaining := file.RemainingAgainstProse(prose)
		checked++
		report.check(fmt.Sprintf("task list %s loads and validates (%d tasks, %d remaining)",
			slug, len(file.Tasks), len(remaining)), true, "")

		// Open acceptance boxes beside a done status are a hard failure when
		// prose exists. Missing prose stays a note: older runs may lack TASKS.md,
		// while task_complete still refuses to settle done without proof.
		if prosePath == "" {
			continue
		}
		if incomplete := file.IncompleteDone(prose); len(incomplete) > 0 {
			ids := make([]string, 0, len(incomplete))
			for _, task := range incomplete {
				ids = append(ids, task.ID)
			}
			msg := fmt.Sprintf("%s marked done with open or missing acceptance criteria: %s",
				slug, strings.Join(ids, ", "))
			// Active delivery must not ignore open AC boxes. Finished runs keep
			// a note so historical task lists do not fail doctor forever.
			if runIsActive(workspaceRoot, slug) {
				report.check(
					fmt.Sprintf("task list %s done tasks have acceptance checkboxes checked", slug),
					false,
					msg,
				)
				continue
			}
			fmt.Printf("  note  %s\n", msg)
		}
	}
	if checked == 0 {
		fmt.Printf("  note  no %s in this workspace; tasks_remaining needs one where a graph reads it\n", tasks.FileName)
	}
}

// runIsActive reports whether a slug still sits in an open delivery loop.
func runIsActive(workspaceRoot, slug string) bool {
	run, err := state.Load(state.ManifestPath(workspaceRoot, slug))
	if err != nil {
		return false
	}
	switch run.Status {
	case state.StatusRunning, state.StatusAwaitingHuman:
		return true
	default:
		return false
	}
}

// checkAutoOptIn reports whether this workspace has answered the auto-mode
// question, and refuses a file it cannot read.
//
// A malformed opt-in must not be reported as an absent one. Absence is a
// deliberate no; a typo is a question nobody answered, and the two should not
// look alike in a health check.
func checkAutoOptIn(report *diagnostics, workspaceRoot string) {
	config, present, err := autoconfig.Load(workspaceRoot)
	if err != nil {
		report.check("auto opt-in loads and validates", false, err.Error())
		return
	}
	if !present {
		fmt.Printf("  note  no %s; auto mode stops at a green pull request\n", autoconfig.FileName)
		return
	}
	report.check("auto opt-in loads and validates", true, "")
	if config.MayMerge() {
		fmt.Printf("  note  auto mode may merge in this workspace, answered in %s\n",
			autoconfig.Path(workspaceRoot))
		return
	}
	fmt.Printf("  note  auto mode may not merge; %s still says merge: false\n", autoconfig.FileName)
}

// sandboxIsolationNote explains that vibe "sandbox" is a runner port, not a
// Claude-class OS Bash sandbox. Doctor prints it when a workspace opted in so
// operators do not confuse local/docker drivers with Seatbelt/bubblewrap.
const sandboxIsolationNote = "sandbox.yaml is a runner port: local = no isolation; docker = bind-mount container (not Claude OS Seatbelt/bubblewrap)"

// checkSandboxConfig notes whether the workspace opted into runner drivers.
// Absence is fine: checks without runner: keep running on the host.
func checkSandboxConfig(report *diagnostics, workspaceRoot string) {
	cfg, present, err := sandbox.Load(workspaceRoot)
	if err != nil {
		report.check("sandbox config loads and validates", false, err.Error())
		return
	}
	if !present {
		fmt.Printf("  note  no %s; checks without runner: stay on the host\n", sandbox.FileName)
		return
	}
	report.check(fmt.Sprintf("sandbox config loads and validates (%d runners)", len(cfg.Spec.Runners)), true, "")
	fmt.Printf("  note  %s\n", sandboxIsolationNote)
}
