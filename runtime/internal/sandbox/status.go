package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// Status values written to STATUS.md.
const (
	StatusUp     = "up"
	StatusDown   = "down"
	StatusFailed = "failed"
)

// StatusFile is the basename under sandbox/<use-case>/.
const StatusFile = "STATUS.md"

// StatusDir returns .agent-state/runs/.../sandbox/<use-case>/.
func StatusDir(workspaceRoot, slug, useCase string) (string, error) {
	runDir := state.RunDir(workspaceRoot, slug)
	if runDir == "" {
		return "", fmt.Errorf("no run directory for slug %q", slug)
	}
	safe := sanitizeUseCase(useCase)
	if safe == "" {
		return "", fmt.Errorf("use case %q is not a valid path segment", useCase)
	}
	return filepath.Join(runDir, "sandbox", safe), nil
}

// StatusPath is STATUS.md for a use case on a run.
func StatusPath(workspaceRoot, slug, useCase string) (string, error) {
	dir, err := StatusDir(workspaceRoot, slug, useCase)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StatusFile), nil
}

// Status is the on-disk record for one use-case runner.
type Status struct {
	State     string
	Runner    string
	Container string
	Updated   time.Time
}

// WriteStatus creates the sandbox dir and writes STATUS.md.
func WriteStatus(workspaceRoot, slug, useCase string, st Status) error {
	dir, err := StatusDir(workspaceRoot, slug, useCase)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create sandbox status dir: %w", err)
	}
	if st.Updated.IsZero() {
		st.Updated = time.Now().UTC()
	}
	body := fmt.Sprintf("status: %s\nrunner: %s\ncontainer: %s\nupdated: %s\n",
		st.State, st.Runner, st.Container, st.Updated.UTC().Format(time.RFC3339))
	path := filepath.Join(dir, StatusFile)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write sandbox status: %w", err)
	}
	return nil
}

// ReadStatus parses STATUS.md when present.
func ReadStatus(workspaceRoot, slug, useCase string) (Status, bool, error) {
	path, err := StatusPath(workspaceRoot, slug, useCase)
	if err != nil {
		return Status{}, false, err
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if os.IsNotExist(err) {
		return Status{}, false, nil
	}
	if err != nil {
		return Status{}, false, err
	}
	st := Status{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "status":
			st.State = value
		case "runner":
			st.Runner = value
		case "container":
			st.Container = value
		case "updated":
			if t, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
				st.Updated = t
			}
		}
	}
	return st, true, nil
}

func sanitizeUseCase(useCase string) string {
	useCase = strings.TrimSpace(useCase)
	if useCase == "" {
		return ""
	}
	for _, r := range useCase {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return ""
	}
	return useCase
}
