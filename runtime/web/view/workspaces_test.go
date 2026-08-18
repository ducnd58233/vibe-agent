package view

import (
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

func TestSplitWorkspacesKeepsCurrentOutOfRecent(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	reg := domain.NewRegistry(rootA, []string{rootB})
	current, recent := SplitWorkspaces(ProjectWorkspaces(reg, rootA))
	if current.Path != filepath.Clean(rootA) || !current.Active {
		t.Fatalf("current = %+v", current)
	}
	if current.Label != filepath.Base(rootA) {
		t.Fatalf("label = %q", current.Label)
	}
	if len(recent) != 1 {
		t.Fatalf("recent = %d", len(recent))
	}
	if recent[0].Path != filepath.Clean(rootB) || recent[0].Active {
		t.Fatalf("recent = %+v", recent[0])
	}
}
