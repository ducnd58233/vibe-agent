// Package checkplan reads the workspace's declaration of how each check is
// produced.
//
// It exists to close a gap the rest of the runtime could not close on its own.
// Run state refuses evidence without provenance, and the verifiers only ever
// report what a process or a file actually did. But until this package, the
// command that produced that exit code was chosen by whoever typed the
// checkpoint. "The unit check exited 0" is a true statement about `true`.
//
// So the command moves into a file the workspace tracks in git. A check runs
// what the repository says it runs. Substituting something weaker is then a diff
// on a reviewed file rather than an argument nobody sees, which is the whole
// difference between a policy and a habit.
//
// The plan lives at the workspace root, not in the toolkit: the commands are
// properties of the project being built, and one toolkit serves many projects.
package checkplan

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// APIVersion and Kind are the only values this loader accepts. They match the
// graph loader's pair so one workspace does not carry two conventions.
const (
	APIVersion = "vibe-agent/v1"
	Kind       = "CheckPlan"
)

// FileName is the plan's name at the workspace root.
const FileName = "vibe-checks.yaml"

// HumanVerifier is the verifier value for a check no runtime verifier can
// produce, where a person decides instead.
//
// It is the only route by which caller-supplied evidence reaches a verifier
// node, so it deliberately costs a tracked diff to grant. A workspace that finds
// itself declaring many of these has an honesty problem worth looking at, not a
// configuration problem.
const HumanVerifier = "human"

// DefaultTimeout bounds an entry that did not set its own.
const DefaultTimeout = 30 * time.Minute

// Entry is how one check is produced.
type Entry struct {
	// Verifier overrides the graph node's verifier kind. Empty means the node
	// decides, which is the normal case.
	Verifier string `yaml:"verifier"`

	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`

	// Paths is for the files verifier.
	Paths []string `yaml:"paths"`

	TimeoutSeconds int `yaml:"timeoutSeconds"`

	// Description is for humans reading the plan. The runtime ignores it.
	Description string `yaml:"description"`
}

// Timeout is the entry's bound, or the package default.
func (e Entry) Timeout() time.Duration {
	if e.TimeoutSeconds <= 0 {
		return DefaultTimeout
	}
	return time.Duration(e.TimeoutSeconds) * time.Second
}

// Human reports whether a person decides this check rather than a verifier.
func (e Entry) Human() bool { return e.Verifier == HumanVerifier }

// runnable reports whether the entry says how to produce anything at all. A
// human-decided check is complete with no command, since the person is the
// mechanism.
func (e Entry) runnable() bool {
	return e.Human() || e.Command != "" || len(e.Paths) > 0
}

// Spec is the plan itself.
type Spec struct {
	Checks map[string]Entry `yaml:"checks"`
}

// Metadata is for humans. Nothing routes on it.
type Metadata struct {
	Description string `yaml:"description"`
}

// Plan is a loaded check plan.
type Plan struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`

	path string
}

// Path is where this plan was read from, for error messages that name a file the
// human can open.
func (p *Plan) Path() string { return p.path }

// Entry returns how a check is produced.
//
// An undeclared check is an error, never a zero value. A zero Entry would run
// nothing and report nothing, and a caller that ignored the error would read
// that as a check with no problems.
func (p *Plan) Entry(name string) (Entry, error) {
	entry, ok := p.Spec.Checks[name]
	if !ok {
		return Entry{}, fmt.Errorf("check %q is not declared in %s; declared checks are %v",
			name, p.path, p.Names())
	}
	return entry, nil
}

// Names lists the declared checks in a stable order.
func (p *Plan) Names() []string {
	names := make([]string, 0, len(p.Spec.Checks))
	for name := range p.Spec.Checks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultPath is the plan's location for a workspace.
func DefaultPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, FileName)
}

// Load reads and validates a plan.
//
// A missing file is an error rather than an empty plan. Treating absence as "no
// checks declared" would make the first repo to forget the file the one where
// every check resolves to nothing, which is the exact failure this package
// exists to prevent.
func Load(path string) (*Plan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no check plan at %s; declare how each check is produced there before verifying", path)
		}
		return nil, fmt.Errorf("read check plan %s: %w", path, err)
	}

	plan, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("check plan %s: %w", path, err)
	}
	plan.path = path
	return plan, nil
}

// Parse decodes and validates plan bytes.
//
// Unknown fields are an error: a misspelled key would otherwise be dropped, and
// the check would run with a default nobody chose.
func Parse(raw []byte) (*Plan, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)

	var plan Plan
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if err := plan.validate(); err != nil {
		return nil, err
	}
	return &plan, nil
}

func (p *Plan) validate() error {
	if p.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion %q is not supported, want %q", p.APIVersion, APIVersion)
	}
	if p.Kind != Kind {
		return fmt.Errorf("kind %q is not a check plan, want %q", p.Kind, Kind)
	}
	if len(p.Spec.Checks) == 0 {
		return fmt.Errorf("spec.checks is empty; a plan that declares no checks cannot verify anything")
	}
	for _, name := range p.Names() {
		entry := p.Spec.Checks[name]
		if !entry.runnable() {
			return fmt.Errorf("check %q declares no command and no paths; there is nothing to run", name)
		}
	}
	return nil
}
