package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
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

// checkTaskFiles validates every task list in the workspace and reports where
// one disagrees with the prose file it mirrors.
//
// The disagreement is a note, not a failure. TASKS.md legitimately carries
// context the JSON does not, and refusing on a count would make the prose file
// answer to the machine one rather than the other way round.
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
		checked++
		report.check(fmt.Sprintf("task list %s loads and validates (%d tasks, %d remaining)",
			slug, len(file.Tasks), len(file.Remaining())), true, "")

		prose := filepath.Join(workspace.DocsDir(workspaceRoot, slug), "TASKS.md")
		raw, readErr := os.ReadFile(filepath.Clean(prose))
		if readErr != nil {
			fmt.Printf("  note  %s has no TASKS.md beside its task list\n", slug)
			continue
		}
		if headings := len(taskHeading.FindAllString(string(raw), -1)); headings != len(file.Tasks) {
			fmt.Printf("  note  %s: TASKS.md names %d task(s), %s holds %d; the prose file may carry context the list does not\n",
				slug, headings, tasks.FileName, len(file.Tasks))
		}
	}
	if checked == 0 {
		fmt.Printf("  note  no %s in this workspace; tasks_remaining needs one where a graph reads it\n", tasks.FileName)
	}
}
