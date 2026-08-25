// Package sandbox is the runner port: workspace-opted drivers that execute
// commands outside (or on) the host without embedding a container runtime in
// the Go process.
//
// Embedded Docker/GPU sandboxes stay declined in AGENTS.md. This package only
// orchestrates external drivers the workspace declares in sandbox.yaml.
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	APIVersion = "vibe-agent/v1"
	Kind       = "SandboxConfig"
	FileName   = "sandbox.yaml"
)

// Path is .agent-state/sandbox.yaml under the workspace.
func Path(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".agent-state", FileName)
}

// Config is the workspace opt-in for runners.
type Config struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       Spec   `yaml:"spec"`
	path       string `yaml:"-"`
}

// Spec holds defaults, named runners, and use-case routing.
type Spec struct {
	DefaultRunner string                `yaml:"defaultRunner"`
	Runners       map[string]RunnerSpec `yaml:"runners"`
	UseCases      map[string]string     `yaml:"useCases"`
}

// RunnerSpec names a driver and optional docker settings.
type RunnerSpec struct {
	Driver  string `yaml:"driver"`
	Image   string `yaml:"image"`
	Workdir string `yaml:"workdir"`
}

// Load reads and validates sandbox.yaml. found is false when the file is absent.
func Load(workspaceRoot string) (cfg Config, found bool, err error) {
	path := Path(workspaceRoot)
	raw, readErr := os.ReadFile(filepath.Clean(path))
	if os.IsNotExist(readErr) {
		return Config{}, false, nil
	}
	if readErr != nil {
		return Config{}, false, readErr
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, true, fmt.Errorf("%s: %w", path, err)
	}
	cfg.path = path
	if err := cfg.validate(); err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

func (c Config) validate() error {
	if c.APIVersion != APIVersion {
		return fmt.Errorf("%s: apiVersion %q, want %q", c.path, c.APIVersion, APIVersion)
	}
	if c.Kind != Kind {
		return fmt.Errorf("%s: kind %q, want %q", c.path, c.Kind, Kind)
	}
	if len(c.Spec.Runners) == 0 {
		return fmt.Errorf("%s: spec.runners must name at least one runner", c.path)
	}
	for name, runner := range c.Spec.Runners {
		switch runner.Driver {
		case "local", "docker":
		default:
			return fmt.Errorf("%s: runner %q has unknown driver %q", c.path, name, runner.Driver)
		}
		if runner.Driver == "docker" && runner.Image == "" {
			return fmt.Errorf("%s: runner %q (docker) needs image", c.path, name)
		}
	}
	if c.Spec.DefaultRunner != "" {
		if _, ok := c.Spec.Runners[c.Spec.DefaultRunner]; !ok {
			return fmt.Errorf("%s: defaultRunner %q is not in runners", c.path, c.Spec.DefaultRunner)
		}
	}
	for useCase, runnerName := range c.Spec.UseCases {
		if _, ok := c.Spec.Runners[runnerName]; !ok {
			return fmt.Errorf("%s: useCase %q names unknown runner %q", c.path, useCase, runnerName)
		}
	}
	return nil
}

// ResolveRunner picks the runner name for a use case or an explicit runner override.
func (c Config) ResolveRunner(useCase, explicitRunner string) (string, RunnerSpec, error) {
	name := explicitRunner
	if name == "" {
		name = c.Spec.UseCases[useCase]
	}
	if name == "" {
		name = c.Spec.DefaultRunner
	}
	if name == "" {
		return "", RunnerSpec{}, fmt.Errorf("%s: no runner for use case %q (set runner, useCases, or defaultRunner)", c.path, useCase)
	}
	spec, ok := c.Spec.Runners[name]
	if !ok {
		return "", RunnerSpec{}, fmt.Errorf("%s: unknown runner %q", c.path, name)
	}
	if spec.Workdir == "" && spec.Driver == "docker" {
		spec.Workdir = "/workspace"
	}
	return name, spec, nil
}
