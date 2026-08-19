package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeRootFindsModule(t *testing.T) {
	root := RuntimeRoot(t)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod missing under %q: %v", root, err)
	}
}

func TestToolkitRootHasAIAgents(t *testing.T) {
	root := ToolkitRoot(t)
	if _, err := os.Stat(filepath.Join(root, ".ai-agents", "commands", "ROUTER.md")); err != nil {
		t.Fatalf(".ai-agents router missing under %q: %v", root, err)
	}
}
