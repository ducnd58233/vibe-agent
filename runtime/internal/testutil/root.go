package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// RuntimeRoot returns the directory that contains runtime/go.mod.
func RuntimeRoot(t *testing.T) string {
	t.Helper()
	return findModuleRoot(t, "github.com/ducnd58233/vibe-agent/runtime")
}

// ToolkitRoot returns the consumer repo root that owns .ai-agents (parent of runtime/).
func ToolkitRoot(t *testing.T) string {
	t.Helper()
	return filepath.Dir(RuntimeRoot(t))
}

func findModuleRoot(t *testing.T, modulePath string) string {
	t.Helper()
	start, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := start
	for {
		modPath := filepath.Clean(filepath.Join(dir, "go.mod"))
		data, err := os.ReadFile(modPath)
		if err == nil && strings.Contains(string(data), "module "+modulePath) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			_, file, _, ok := runtime.Caller(2)
			if ok {
				t.Fatalf("module root %q not found from %s (started at %s)", modulePath, file, start)
			}
			t.Fatalf("module root %q not found from %s", modulePath, start)
		}
		dir = parent
	}
}
