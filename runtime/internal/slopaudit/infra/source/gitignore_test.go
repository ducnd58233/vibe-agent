package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScannerSkipsGitignoredPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("/ignored/\n/secret.env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ignoredDir := filepath.Join(dir, "ignored", "nested")
	if err := os.MkdirAll(ignoredDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "bad.js"), []byte("console.log('debug temporary')"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.env"), []byte("TODO leak"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.js"), []byte("console.log('debug temporary')"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range result.Findings {
		rel, err := filepath.Rel(dir, finding.Path)
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "ignored/") || rel == "secret.env" {
			t.Fatalf("gitignored path scanned: %+v", finding)
		}
	}
	if result.Summary.Languages["JavaScript"] != 1 {
		t.Fatalf("summary = %+v", result.Summary)
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
