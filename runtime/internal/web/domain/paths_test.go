package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestFormatAttachAbsQuotesSpaceAndRejectsRelative(t *testing.T) {
	root := t.TempDir()
	spaced := filepath.Join(root, "my file.txt")
	if err := os.WriteFile(spaced, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := FormatAttachAbs(spaced)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "@") {
		t.Fatalf("attach must not use @: %q", got)
	}
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("path with space must be quoted: %q", got)
	}
	inner := strings.Trim(got, `"`)
	if strings.Contains(inner, `\`) {
		t.Fatalf("expected slash path: %q", got)
	}
	if !filepath.IsAbs(filepath.FromSlash(inner)) {
		t.Fatalf("expected absolute path: %q", got)
	}
	if _, err := FormatAttachAbs("rel/file.txt"); err == nil {
		t.Fatal("expected error for relative path")
	}
	if _, err := FormatAttachAbs(filepath.ToSlash(root) + "/../outside"); err == nil {
		t.Fatal("expected error for ..")
	}
}
