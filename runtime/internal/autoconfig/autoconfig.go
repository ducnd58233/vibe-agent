// Package autoconfig reads a workspace's opt-in for auto mode.
//
// Auto mode may record its own merge approval, which is the one boundary this
// toolkit used to state without exception. The opt-in is therefore a file a
// person edited and can be read in a diff, not a flag, not an environment
// variable, and not something inferred from a previous run.
//
// Absence is a no. A workspace with no file has not opted in, and neither has
// one whose file still says false.
package autoconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// APIVersion and Kind match the pair every other file in this workspace uses,
// so one workspace does not carry two conventions.
const (
	APIVersion = "vibe-agent/v1"
	Kind       = "AutoConfig"
)

// FileName is the config's name inside the state directory.
const FileName = "auto.yaml"

// Budgets bound one auto run. Zero means no limit, matching run state, so a
// workspace opts into the ceilings it wants rather than inheriting numbers
// somebody picked.
type Budgets struct {
	Tokens           int `yaml:"tokens"`
	WallclockSeconds int `yaml:"wallclockSeconds"`
}

// Spec is the answer set.
type Spec struct {
	// Merge is the whole opt-in. False means auto mode stops at a green pull
	// request and a person merges it.
	Merge   bool    `yaml:"merge"`
	Budgets Budgets `yaml:"budgets"`
}

// Config is the file.
type Config struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       Spec   `yaml:"spec"`

	// digest fingerprints the bytes this was read from. Unexported and set only
	// by Load, so it cannot be filled in by anything that did not read a file.
	digest string
}

// Path is where the opt-in lives for a workspace.
func Path(workspaceRoot string) string {
	return filepath.Join(workspace.StateDir(workspaceRoot), FileName)
}

// MayMerge reports whether this workspace has opted into auto-merge.
//
// It takes the config rather than a path so the decision is visible at the call
// site: a caller that has not loaded a config cannot accidentally get a yes.
func (c *Config) MayMerge() bool {
	return c != nil && c.Spec.Merge
}

// Parse reads and validates a config.
func Parse(raw []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	if config.APIVersion != APIVersion {
		return nil, fmt.Errorf("%s: apiVersion %q, want %q", FileName, config.APIVersion, APIVersion)
	}
	if config.Kind != Kind {
		return nil, fmt.Errorf("%s: kind %q, want %q", FileName, config.Kind, Kind)
	}
	if config.Spec.Budgets.Tokens < 0 || config.Spec.Budgets.WallclockSeconds < 0 {
		return nil, fmt.Errorf("%s: a budget bounds a run, so it cannot be negative", FileName)
	}
	return &config, nil
}

// Load reads a workspace's config. A missing file is reported as absent rather
// than as an error, because not opting in is a normal state.
func Load(workspaceRoot string) (*Config, bool, error) {
	raw, err := os.ReadFile(filepath.Clean(Path(workspaceRoot)))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	config, err := Parse(raw)
	if err != nil {
		return nil, true, err
	}
	config.digest = digestOf(raw)
	return config, true, nil
}

// Digest is a short fingerprint of the bytes this config was read from.
//
// It exists so an approval can record what it was answering rather than only
// where the answer lived. The opt-in file is gitignored, so there is no diff to
// fall back on: an approval that names a path says a person answered yes, while
// the file can say something else by the time anyone reads the manifest. That
// gap is not hypothetical - a run was aborted on it - and the evidence has to
// carry the answer itself to close it.
//
// Set by Load from the bytes it parsed, in the same call, because reading the
// file a second time to fingerprint it would leave exactly the window this is
// about. Empty on a config built any other way, which is honest: nothing was
// read, so nothing can be fingerprinted.
func (c *Config) Digest() string {
	if c == nil {
		return ""
	}
	return c.digest
}

// digestLength keeps the reference to one readable line. A prefix answers the
// only question asked of it, which is whether these are the same bytes.
const digestLength = 16

func digestOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:digestLength]
}

// Template is what `auto init` writes.
//
// The questions live in the file rather than in a prompt. A terminal question
// is answered once by whoever happened to run the command; a file is answered
// in a diff somebody can review, and it is still there to re-read six months
// later. It also means `auto init` works the same when nobody is watching.
const Template = `# Auto mode opt-in for this workspace.
#
# Auto mode drives spec, plan, build, review, test, and CI without stopping for
# a person. This file is where you say how far that is allowed to go.
#
# Nothing here is inferred. A missing file, or merge left false, means auto mode
# stops at a green pull request and a person merges it.

apiVersion: vibe-agent/v1
kind: AutoConfig

spec:
  # May auto mode merge its own pull request to the integration branch?
  #
  # It may only do so when every one of these already holds: required CI checks
  # passed from the API, every test the spec names passed, the linter is clean
  # with nothing suppressed, /ship returned GO, and the diff touches nothing on
  # the danger list. This answer is the last of those conditions, and the only
  # one a machine cannot produce.
  merge: false

  # Ceilings for one auto run. Zero means no limit.
  #
  # An autonomous loop with no ceiling is the failure mode that runs all night.
  # Tokens are host-reported; wallclock is measured from the run's start.
  budgets:
    tokens: 0
    wallclockSeconds: 0
`

// Write puts the template in place. It refuses to overwrite, because the file
// holds an answer somebody gave and regenerating it would silently revoke it.
func Write(workspaceRoot string) (string, error) {
	path := Path(workspaceRoot)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("%s already exists; edit it rather than regenerating, "+
			"which would reset an answer somebody gave", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return path, err
	}
	if err := os.WriteFile(path, []byte(Template), 0o600); err != nil {
		return path, err
	}
	return path, nil
}
