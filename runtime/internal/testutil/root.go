package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const runtimeModule = "github.com/ducnd58233/vibe-agent/runtime"

// RuntimeRoot returns the directory that contains runtime/go.mod.
func RuntimeRoot(tb testing.TB) string {
	tb.Helper()
	return findModuleRoot(tb, runtimeModule)
}

// ToolkitRoot returns the consumer repo root that owns .ai-agents (parent of runtime/).
func ToolkitRoot(tb testing.TB) string {
	tb.Helper()
	return filepath.Dir(RuntimeRoot(tb))
}

func findModuleRoot(tb testing.TB, modulePath string) string {
	tb.Helper()
	start, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
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
				tb.Fatalf("module root %q not found from %s (started at %s)", modulePath, file, start)
			}
			tb.Fatalf("module root %q not found from %s", modulePath, start)
		}
		dir = parent
	}
}
