package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
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
	checkCheckPlan(report, workspaceRoot, toolkitRoot)
	checkMemory(report, workspaceRoot)
	checkRunState(report, workspaceRoot)
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
		var missing []string
		for name, node := range loaded.Spec.Nodes {
			if node.Type != graph.NodeVerifier || node.Check == "" {
				continue
			}
			if _, err := plan.Entry(node.Check); err != nil {
				missing = append(missing, fmt.Sprintf("%s (node %s)", node.Check, name))
			}
		}
		sort.Strings(missing)
		report.check("graph "+id+": every verifier node has a planned check",
			len(missing) == 0,
			"undeclared in "+checkplan.FileName+": "+strings.Join(missing, ", "))
	}
}

func checkMemory(report *diagnostics, workspaceRoot string) {
	store, err := memory.Open(workspaceRoot)
	if err != nil {
		report.check("memory database opens", false, err.Error())
		return
	}
	report.check("memory database opens", true, "")
	store.Close()
}

// checkRunState loads every manifest under tmp/, so a run that would fail to
// resume is found now rather than mid-delivery.
func checkRunState(report *diagnostics, workspaceRoot string) {
	entries, err := os.ReadDir(filepath.Join(workspaceRoot, "tmp"))
	if err != nil {
		return // no runs yet is not a problem
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := state.ManifestPath(workspaceRoot, entry.Name())
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if _, err := state.Load(path); err != nil {
			report.check("run state "+entry.Name()+" is valid", false, err.Error())
			continue
		}
		report.check("run state "+entry.Name()+" is valid", true, "")
	}
}

// checkGitignore catches the mistake that would commit run evidence or a memory
// database, both of which are workspace-local state rather than assets.
func checkGitignore(report *diagnostics, workspaceRoot string) {
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, ".gitignore"))
	if err != nil {
		return // no .gitignore is a consumer choice, not a runtime problem
	}
	content := string(raw)
	report.check("tmp/ is gitignored", strings.Contains(content, "/tmp/"),
		"add /tmp/ so run evidence is not committed")
	report.check(memory.StateDirName+"/ is gitignored",
		strings.Contains(content, "/"+memory.StateDirName+"/"),
		"add /"+memory.StateDirName+"/ so the memory database is not committed")
}
