// Package tasks reads the machine-readable task list a plan produces.
//
// TASKS.md stays the file a person reads and reviews in a pull request. This is
// the file a verifier reads. A task marked done settles only when every
// acceptance-criteria checkbox in that TASKS section is checked; status alone
// is not enough.
//
// The two files are expected to agree on task identity and status. Doctor notes
// a heading-count mismatch and refuses open acceptance boxes on an active run.
package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// SchemaVersion is the only version this loader accepts.
const SchemaVersion = 1

// FileName is the undated legacy basename. Prefer DocsArtifact("tasks", date).
const FileName = "tasks.json"

// Status is how far a task has got.
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

// Settled reports whether a task needs no further work.
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
	Date          string `json:"date"`
	Version       int    `json:"version"`
	Tasks         []Task `json:"tasks"`
}

// Path is where a slug's task list lives. Prefer the dated basename under the
// versioned docs dir; fall back to undated names and the flat docs tree.
func Path(workspaceRoot, slug string) string {
	if entry, err := runpath.Resolve(workspaceRoot, slug); err == nil {
		dir := workspace.DocsDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
		dated, err := workspace.DocsArtifact("tasks", entry.Date)
		if err == nil {
			candidate := filepath.Join(dir, dated)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		legacy := filepath.Join(dir, FileName)
		if _, err := os.Stat(legacy); err == nil {
			return legacy
		}
		if dated != "" {
			return filepath.Join(dir, dated)
		}
		return legacy
	}
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
	if !validate.Slug(file.Slug) {
		return nil, fmt.Errorf("%s: slug %q must be lowercase kebab-case", FileName, file.Slug)
	}
	if !validate.Date(file.Date) {
		return nil, fmt.Errorf("%s: date %q is not YYYY-MM-DD", FileName, file.Date)
	}
	if file.Version < 1 {
		return nil, fmt.Errorf("%s: version must be >= 1, got %d", FileName, file.Version)
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
