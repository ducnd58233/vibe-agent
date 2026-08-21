package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
	"github.com/ducnd58233/vibe-agent/runtime/internal/tasks"
)

// taskPacket answers "what's the next task" in one call, replacing a manual
// re-read of tasks.json and TASKS.md. It never invents a task: an unreadable
// or missing task list is reported as a status, not guessed past.
func taskPacket(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Slug) == "" {
		return nil, fmt.Errorf("vibe_task_packet needs a slug")
	}

	file, err := tasks.Load(deps.WorkspaceRoot, args.Slug)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"status": "no_plan_yet",
				"note":   "no tasks.json yet for this run; the plan node has not produced one",
			}, nil
		}
		return nil, fmt.Errorf("vibe_task_packet %s: %w", args.Slug, err)
	}

	next := nextActionableTask(file.Tasks)
	if next == nil {
		// A JSON-all-done list can still have open acceptance boxes; those are
		// not finished for the delivery loop either.
		prose, _ := tasks.LoadProse(deps.WorkspaceRoot, args.Slug)
		if remaining := file.RemainingAgainstProse(prose); len(remaining) > 0 {
			next = &remaining[0]
		}
	}
	if next == nil {
		return map[string]any{
			"status": "all_done",
			"note":   "every task is done or canceled with acceptance criteria checked",
		}, nil
	}

	task := map[string]any{
		"id": next.ID, "title": next.Title, "status": string(next.Status),
		"branch": next.Branch, "acceptance": next.Acceptance, "dependsOn": next.DependsOn,
	}
	if detail := taskDetail(deps.WorkspaceRoot, args.Slug, next.ID); detail != "" {
		task["acceptanceDetail"] = detail
	}
	return map[string]any{"status": "task_ready", "task": task}, nil
}

// nextActionableTask picks the task already in progress, or the first queued
// task whose dependencies are all settled - the same rule task_complete's own
// verifier and planning-and-task-breakdown both use, so this tool and the
// graph never disagree about what "next" means.
func nextActionableTask(list []tasks.Task) *tasks.Task {
	byID := make(map[string]tasks.Task, len(list))
	for _, task := range list {
		byID[task.ID] = task
	}
	for i := range list {
		if list[i].Status == tasks.StatusInProgress {
			return &list[i]
		}
	}
	for i := range list {
		if list[i].Status != tasks.StatusQueued {
			continue
		}
		ready := true
		for _, dep := range list[i].DependsOn {
			if !byID[dep].Status.Settled() {
				ready = false
				break
			}
		}
		if ready {
			return &list[i]
		}
	}
	return nil
}

// taskDetail reads the fuller acceptance-criteria section from TASKS.md, the
// prose file a person reviews. tasks.json's one-line acceptance field is not
// always the whole story. A missing or unreadable file reports no detail
// rather than an error: the one-line acceptance still answers the call.
func taskDetail(workspaceRoot, slug, taskID string) string {
	entry, err := runpath.Resolve(workspaceRoot, slug)
	if err != nil {
		return ""
	}
	name, err := workspace.DocsArtifact("TASKS", entry.Date)
	if err != nil {
		return ""
	}
	dir := workspace.DocsDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(dir, name)))
	if err != nil {
		return ""
	}
	return taskSection(string(raw), taskID)
}

// taskSection extracts one "## T<id>: ..." heading's body, up to the next
// top-level heading.
func taskSection(markdown, taskID string) string {
	marker := "## " + taskID + ":"
	lines := strings.Split(markdown, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), marker) {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}
