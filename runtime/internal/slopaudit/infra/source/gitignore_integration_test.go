package source

import (
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestLoadGitignoreFromWorkspaceRoot(t *testing.T) {
	root := testutil.ToolkitRoot(t)
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
