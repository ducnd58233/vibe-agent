package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

const workspacesFileName = "web-workspaces.json"

// WorkspacesFileName is persisted beside web.json under .agent-state/.
func WorkspacesFileName() string {
	return workspacesFileName
}

// Registry lists workspace roots one web process may serve.
type Registry struct {
	Default string   `json:"default"`
	Roots   []string `json:"roots"`
}

// NewRegistry deduplicates and cleans workspace roots.
func NewRegistry(defaultRoot string, extra []string) Registry {
	seen := map[string]struct{}{}
	var roots []string
	add := func(raw string) {
		clean := filepath.Clean(strings.TrimSpace(raw))
		if clean == "" || clean == "." {
			return
		}
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		roots = append(roots, clean)
	}
	add(defaultRoot)
	for _, root := range extra {
		add(root)
	}
	def := filepath.Clean(defaultRoot)
	if def == "" && len(roots) > 0 {
		def = roots[0]
	}
	return Registry{Default: def, Roots: roots}
}

// ID is a stable cookie token for a registered root.
func (r Registry) ID(root string) string {
	clean := filepath.Clean(root)
	sum := sha256.Sum256([]byte(clean))
	return hex.EncodeToString(sum[:6])
}

// Resolve maps an ID back to a root when registered.
func (r Registry) Resolve(id string) (string, bool) {
	for _, root := range r.Roots {
		if r.ID(root) == id {
			return root, true
		}
	}
	return "", false
}

// Contains reports whether root is registered.
func (r Registry) Contains(root string) bool {
	clean := filepath.Clean(root)
	for _, item := range r.Roots {
		if item == clean {
			return true
		}
	}
	return false
}
