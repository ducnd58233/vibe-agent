package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestPresentBasenames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := workspace.PresentBasenames(root, "AGENTS.md", "missing.md", "CLAUDE.md")
	if len(got) != 1 || got[0] != "AGENTS.md" {
		t.Fatalf("got %#v", got)
	}
}
