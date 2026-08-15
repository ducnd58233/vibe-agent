package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlopAuditAcceptsFormatFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte("package clean\n\nfunc Value() int { return 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := slopAuditCommand([]string{"--format", "json", dir}); err != nil {
		t.Fatalf("slop audit --format json failed: %v", err)
	}
}

func TestSlopAuditRejectsUnknownFormat(t *testing.T) {
	if err := slopAuditCommand([]string{"--format", "xml"}); err == nil {
		t.Fatal("unknown format was accepted")
	}
}

func TestSlopAuditTakesOnePath(t *testing.T) {
	if err := slopAuditCommand([]string{".", ".."}); err == nil {
		t.Fatal("multiple paths were accepted")
	}
}
