// Package tasks reads the machine-readable task list a plan produces.
//
// TASKS.md stays the file a person reads and reviews in a pull request. This is
// the file a verifier reads, and it exists because "is another task left" was
// the one human verifier in the delivery plan that was human only because
// nothing had given it a machine-readable input. The other two, an external
// review and a ship decision, are genuine judgement calls.
//
// The two files are expected to agree. Nothing here enforces that: doctor
// reports a disagreement in counts and does not refuse, because the prose file
// legitimately carries context the JSON does not.
package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// SchemaVersion is the only version this loader accepts.
const SchemaVersion = 1

// FileName is the task list's name inside a slug's docs directory.
const FileName = "tasks.json"

// Status is how far a task has got.
//
// Five, not more. `ready` and `awaiting_user` are derivable from dependencies
// and from run state, and a status nothing reads is a status that goes stale.
type Status string

const (
	StatusQueued     Status = "queued"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDone       Status = "done"
	StatusCanceled   Status = "canceled"
)

func (s Status) valid() bool {
	switch s {
	case StatusQueued, StatusInProgress, StatusBlocked, StatusDone, StatusCanceled:
		return true
	}
	return false
}

// Settled reports whether a task needs no further work. Only these two end a
// task: a blocked task is still in scope, which is the point of the state.
func (s Status) Settled() bool {
	return s == StatusDone || s == StatusCanceled
}

// Task is one planned unit of work.
type Task struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Status     Status   `json:"status"`
	Branch     string   `json:"branch,omitempty"`
	DependsOn  []string `json:"dependsOn,omitempty"`
	Acceptance string   `json:"acceptance,omitempty"`
	Note       string   `json:"note,omitempty"`
}

// File is the whole task list for one slug.
type File struct {
	SchemaVersion int    `json:"schemaVersion"`
	Slug          string `json:"slug"`
	Tasks         []Task `json:"tasks"`
}

// Path is where a slug's task list lives, beside the TASKS.md it mirrors.
func Path(workspaceRoot, slug string) string {
	return filepath.Join(workspace.DocsDir(workspaceRoot, slug), FileName)
}

// Parse reads and validates a task list.
func Parse(raw []byte) (*File, error) {
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	if file.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("%s: schemaVersion %d, want %d", FileName, file.SchemaVersion, SchemaVersion)
	}
	if len(file.Tasks) == 0 {
		return nil, fmt.Errorf("%s: no tasks; an empty list would end a run rather than describe one", FileName)
	}
	seen := map[string]bool{}
	for index, task := range file.Tasks {
		switch {
		case task.ID == "":
			return nil, fmt.Errorf("%s: task %d has no id", FileName, index)
		case seen[task.ID]:
			return nil, fmt.Errorf("%s: task %q is listed twice", FileName, task.ID)
		case task.Title == "":
			return nil, fmt.Errorf("%s: task %q has no title", FileName, task.ID)
		case !task.Status.valid():
			return nil, fmt.Errorf("%s: task %q has status %q; use queued, in_progress, blocked, done, or canceled",
				FileName, task.ID, task.Status)
		}
		seen[task.ID] = true
	}
	for _, task := range file.Tasks {
		for _, dependency := range task.DependsOn {
			if !seen[dependency] {
				return nil, fmt.Errorf("%s: task %q depends on %q, which is not in the list",
					FileName, task.ID, dependency)
			}
		}
	}
	return &file, nil
}

// Load reads a slug's task list from disk.
func Load(workspaceRoot, slug string) (*File, error) {
	raw, err := os.ReadFile(filepath.Clean(Path(workspaceRoot, slug)))
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Remaining returns the tasks still in scope, in list order.
func (f *File) Remaining() []Task {
	var out []Task
	for _, task := range f.Tasks {
		if !task.Status.Settled() {
			out = append(out, task)
		}
	}
	return out
}
