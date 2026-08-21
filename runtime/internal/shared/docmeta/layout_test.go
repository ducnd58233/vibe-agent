package docmeta_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/docmeta"
)

func TestCheckWorkspaceFailsFlatSPEC(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "legacy-slug")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte("# x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	issues, err := docmeta.CheckWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("flat SPEC.md produced no issue")
	}
	if !strings.Contains(issues[0].Message, "flat") {
		t.Fatalf("issue = %+v", issues[0])
	}
}

func TestCheckWorkspaceFailsMissingFrontMatter(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "2026-08-21", "demo", "1")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SPEC-2026-08-21.md"), []byte("# no front matter\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	issues, err := docmeta.CheckWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("missing front matter produced no issue")
	}
	if !strings.Contains(issues[0].Message, "front matter") {
		t.Fatalf("issue = %+v", issues[0])
	}
}

func TestCheckWorkspaceAcceptsValidRevision(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "2026-08-21", "demo", "1")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte("---\nslug: demo\ndate: 2026-08-21\nversion: 1\n---\n\n# Spec\n")
	if err := os.WriteFile(filepath.Join(dir, "SPEC-2026-08-21.md"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	issues, err := docmeta.CheckWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
}
