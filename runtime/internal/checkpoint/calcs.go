package checkpoint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/calc"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// calcTargets names, per graph and node, the documents whose logged
// calculations must recompute before the run leaves that node. A stem is the
// part of the file name before the date: RESEARCH is RESEARCH-<date>.md.
//
// A node appears here when its artifact is the one a person or a later node
// will rely on for a figure: the digest, the spec, the plan, the findings, the
// write-up, and a learner's study record, whose answer keys are computed.
var calcTargets = map[string]map[string][]string{
	"goal-delivery": {
		"research":      {"RESEARCH"},
		"auto_research": {"RESEARCH"},
		"spec":          {"SPEC"},
		"plan":          {"PLAN", "TASKS"},
	},
	"researcher-delivery": {
		"literature":        {"RESEARCH"},
		"hypothesis":        {"HYPOTHESIS"},
		"experiment_design": {"PLAN", "TASKS"},
		"findings":          {"FINDINGS"},
		"writeup":           {"WRITEUP"},
	},
	"task-delivery": {
		"research": {"RESEARCH"},
		"spec":     {"SPEC"},
	},
	"study-delivery": {
		"study_plan": {"STUDY"},
		"session":    {"STUDY"},
	},
}

// checkCalcs refuses to leave a node while a document it just wrote logs a
// calculation that does not recompute. It needs no network, so unlike the
// citation check it never fails closed for any reason but a wrong figure.
//
// A document with no calc block passes: the rule is that a figure the document
// computed is logged and checkable, and one that computed nothing owes nothing.
// A document that does not exist yet passes too, because the host may
// checkpoint before the file is written; the node's own output check is what
// asks whether it exists.
func checkCalcs(workspaceRoot string, run *state.Run) error {
	stems := calcTargets[run.GraphID][run.CurrentNode]
	if len(stems) == 0 || run.Date == "" || run.Version < 1 {
		return nil
	}
	dir := sharedworkspace.DocsDirAt(workspaceRoot, run.Date, run.Slug, run.Version)

	var lines []string
	failed := 0
	for _, stem := range stems {
		name := stem + "-" + run.Date + ".md"
		raw, err := os.ReadFile(filepath.Clean(filepath.Join(dir, name)))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		_, _, issues := calc.CheckMarkdown(raw)
		for _, issue := range issues {
			lines = append(lines, "  "+name+": "+issue.String())
		}
		failed += len(issues)
	}
	if failed == 0 {
		return nil
	}
	return fmt.Errorf("%s logs %d calculation(s) that do not recompute; fix each figure, or the line, before continuing:\n%s",
		run.CurrentNode, failed, strings.Join(lines, "\n"))
}
