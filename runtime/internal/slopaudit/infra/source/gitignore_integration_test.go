package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGitignoreFromWorkspaceRoot(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err != nil {
		t.Skip("workspace root not found from runtime module")
	}
	g := loadGitignore(root)
	agentState := filepath.Join(root, ".agent-state")
	if !g.skipDir(agentState) {
		t.Fatalf("expected .agent-state to be gitignored at %s", agentState)
	}
	fetchFile := filepath.Join(root, ".agent-state", "fetch", "example.json")
	if !g.skipFile(fetchFile) {
		t.Fatalf("expected file under .agent-state to be gitignored")
	}
	tmpFile := filepath.Join(root, "tmp", "probe", "x.log")
	if !g.skipFile(tmpFile) {
		t.Fatalf("expected file under tmp/ to be gitignored")
	}
}
