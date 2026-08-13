// Package state persists one goal run.
//
// Run state lives at tmp/<slug>/manifest.json beside the human-readable
// RECORD.md, and the append-only event log at tmp/<slug>/events.ndjson. Both
// are gitignored. The shapes match schemas/run-state.schema.json.
//
// The rule this package exists to enforce: a check is passed only when real
// evidence produced it. There is no provenance value for model assertion, so
// no caller can mark its own work complete.
package domain

import (
	"fmt"
	"regexp"
	"time"
)

// SchemaVersion is the manifest shape this package reads and writes.
const SchemaVersion = 1

// CheckSource is where a check result came from. Every value here is something
// outside the model: a process, a file, an API, or a person.
type CheckSource string

const (
	SourceExitCode   CheckSource = "exit_code"
	SourceFileAssert CheckSource = "file_assert"
	SourceCIAPI      CheckSource = "ci_api"
	SourceHumanEvent CheckSource = "human_event"
)

func (s CheckSource) valid() bool {
	switch s {
	case SourceExitCode, SourceFileAssert, SourceCIAPI, SourceHumanEvent:
		return true
	}
	return false
}

// Status is where a run stands.
type Status string

const (
	StatusRunning        Status = "running"
	StatusAwaitingHuman  Status = "awaiting_human"
	StatusDone           Status = "done"
	StatusFailed         Status = "failed"
	StatusCancelled      Status = "cancelled"
	StatusBudgetExceeded Status = "budget_exceeded"
)

func (s Status) valid() bool {
	switch s {
	case StatusRunning, StatusAwaitingHuman, StatusDone,
		StatusFailed, StatusCancelled, StatusBudgetExceeded:
		return true
	}
	return false
}

// Check is one verification result and the evidence behind it.
type Check struct {
	Passed bool `json:"passed"`
	// Skipped marks a check that did not run. A skipped check is not a passed
	// check; only a guard that explicitly accepts both may treat them alike.
	Skipped  bool        `json:"skipped,omitempty"`
	Source   CheckSource `json:"source"`
	Ref      string      `json:"ref,omitempty"`
	ExitCode *int        `json:"exitCode,omitempty"`
	At       time.Time   `json:"at"`
}

func (c Check) validate(name string) error {
	if !c.Source.valid() {
		return fmt.Errorf("check %q: source %q is not evidence; use one of exit_code, file_assert, ci_api, human_event", name, c.Source)
	}
	if c.Passed && c.Skipped {
		return fmt.Errorf("check %q: cannot be both passed and skipped", name)
	}
	return nil
}

// Blocker records a failure that keeps recurring at the same node.
type Blocker struct {
	Node   string `json:"node"`
	Reason string `json:"reason"`
	// Attempts drives the stop rule: three cycles on the same blocker ends the
	// run and reports root cause instead of retrying a fourth time.
	Attempts int       `json:"attempts"`
	At       time.Time `json:"at"`
}

// Artifact is a file a node produced.
type Artifact struct {
	Path string    `json:"path"`
	Node string    `json:"node"`
	At   time.Time `json:"at,omitempty"`
}

// Run is the machine-readable state of one goal run.
type Run struct {
	SchemaVersion int    `json:"schemaVersion"`
	RunID         string `json:"runId"`
	GraphID       string `json:"graphId"`
	Slug          string `json:"slug"`
	Goal          string `json:"goal"`

	CurrentNode    string `json:"currentNode"`
	Status         Status `json:"status"`
	Iteration      int    `json:"iteration"`
	MaxTransitions int    `json:"maxTransitions"`

	Branch    *string `json:"branch"`
	PR        *string `json:"pr"`
	TaskIndex *int    `json:"taskIndex,omitempty"`
	TaskCount *int    `json:"taskCount,omitempty"`

	Flags     map[string]bool  `json:"flags,omitempty"`
	Checks    map[string]Check `json:"checks,omitempty"`
	Blockers  []Blocker        `json:"blockers,omitempty"`
	Artifacts []Artifact       `json:"artifacts,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// NewRun starts a run. The id embeds the UTC start time so runs sort
// chronologically, and the slug so a directory listing is readable.
func NewRun(slug, goal, graphID string, maxTransitions int, now time.Time) (*Run, error) {
	if !slugPattern.MatchString(slug) {
		return nil, fmt.Errorf("slug %q must be lowercase kebab-case", slug)
	}
	if goal == "" {
		return nil, fmt.Errorf("goal must not be empty")
	}
	if graphID == "" {
		return nil, fmt.Errorf("graphId must not be empty")
	}
	if maxTransitions < 1 {
		return nil, fmt.Errorf("maxTransitions must be at least 1, got %d", maxTransitions)
	}

	now = now.UTC()
	return &Run{
		SchemaVersion:  SchemaVersion,
		RunID:          fmt.Sprintf("run_%s_%s", now.Format("2006-01-02T15-04-05Z"), slug),
		GraphID:        graphID,
		Slug:           slug,
		Goal:           goal,
		CurrentNode:    "",
		Status:         StatusRunning,
		Iteration:      0,
		MaxTransitions: maxTransitions,
		Flags:          map[string]bool{},
		Checks:         map[string]Check{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// SetCheck records a verification result, rejecting anything without real
// provenance. Use SetCheckAt to control the UpdatedAt stamp.
func (r *Run) SetCheck(name string, check Check) error {
	return r.SetCheckAt(name, check, check.At)
}

// SetCheckAt records a check and stamps UpdatedAt with now.
func (r *Run) SetCheckAt(name string, check Check, now time.Time) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("check name %q must be lowercase with underscores", name)
	}
	if err := check.validate(name); err != nil {
		return err
	}
	if check.At.IsZero() {
		check.At = now
	}
	check.At = check.At.UTC()

	if r.Checks == nil {
		r.Checks = map[string]Check{}
	}
	r.Checks[name] = check
	r.UpdatedAt = now.UTC()
	return nil
}

// SetFlagAt records a scope decision and stamps UpdatedAt.
//
// Flags carry no provenance field, unlike checks, because they are not evidence
// about the work: they say which parts of the graph apply. Who may write one is
// therefore a question for the surface that offers it, not for this struct. The
// name is validated here so a flag can never be stored under a key no guard
// could be declared with.
func (r *Run) SetFlagAt(name string, value bool, now time.Time) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("flag name %q must be lowercase with underscores", name)
	}
	if r.Flags == nil {
		r.Flags = map[string]bool{}
	}
	r.Flags[name] = value
	r.UpdatedAt = now.UTC()
	return nil
}

// Validate reports whether the run is internally consistent. Load applies it to
// every manifest it reads, so a hand-edited file gets the same scrutiny as one
// this package wrote.
func (r *Run) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schemaVersion %d is not supported, want %d", r.SchemaVersion, SchemaVersion)
	}
	if !slugPattern.MatchString(r.Slug) {
		return fmt.Errorf("slug %q must be lowercase kebab-case", r.Slug)
	}
	if r.RunID == "" || r.GraphID == "" || r.Goal == "" {
		return fmt.Errorf("runId, graphId, and goal are all required")
	}
	if !r.Status.valid() {
		return fmt.Errorf("status %q is not a known status", r.Status)
	}
	if r.Iteration < 0 {
		return fmt.Errorf("iteration must not be negative, got %d", r.Iteration)
	}
	if r.MaxTransitions < 1 {
		return fmt.Errorf("maxTransitions must be at least 1, got %d", r.MaxTransitions)
	}
	for name := range r.Flags {
		if !namePattern.MatchString(name) {
			return fmt.Errorf("flag name %q must be lowercase with underscores", name)
		}
	}
	for name, check := range r.Checks {
		if !namePattern.MatchString(name) {
			return fmt.Errorf("check name %q must be lowercase with underscores", name)
		}
		if err := check.validate(name); err != nil {
			return err
		}
	}
	for i, blocker := range r.Blockers {
		if blocker.Node == "" || blocker.Reason == "" {
			return fmt.Errorf("blocker %d needs a node and a reason", i)
		}
		if blocker.Attempts < 1 {
			return fmt.Errorf("blocker %d needs at least one attempt, got %d", i, blocker.Attempts)
		}
	}
	return nil
}
