package sourcefiles

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

func TestGitignoreMatchesAnchoredDirectory(t *testing.T) {
	root := t.TempDir()
	g := gitignore{
		root: root,
		patterns: []pattern{
			{raw: ".agents/skills", anchored: true, dirOnly: true},
		},
	}
	skillsDir := filepath.Join(root, ".agents", "skills")
	if !g.skipDir(skillsDir) {
		t.Fatal("expected .agents/skills directory to be skipped")
	}
	if g.skipFile(filepath.Join(root, "runtime", "main.go")) {
		t.Fatal("did not expect runtime file to be skipped")
	}
}
