package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocsCheckCalcs(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.md")
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(good, []byte("```calc\n1 + 1 => 2\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("```calc\n1 + 1 => 3\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := docsCheckCalcs([]string{good}); err != nil {
		t.Errorf("a true calc block failed: %v", err)
	}
	if err := docsCheckCalcs([]string{good, bad}); err == nil {
		t.Error("a false calc block passed")
	}
	if err := docsCheckCalcs(nil); err == nil {
		t.Error("no file was accepted")
	}
}
