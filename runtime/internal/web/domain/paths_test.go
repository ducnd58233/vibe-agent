package domain

import "testing"

func TestResolveWorkspacePathRejectsParent(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveWorkspacePath(root, "../outside"); err == nil {
		t.Fatal("expected error for ..")
	}
}

func TestResolveWorkspacePathAllowsNested(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveWorkspacePath(root, "src/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if got == root {
		t.Fatal("expected nested file path")
	}
}
