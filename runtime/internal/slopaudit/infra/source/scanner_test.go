package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

func TestScannerFindsSignalsAcrossLanguages(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": `package main
import "fmt"
func Work() {
	_ = risky()
	fmt.Println("debug temporary")
	panic("TODO not implemented")
}
func Empty() {}
func risky() error { return nil }
func Swallow() error {
	_, err := fmt.Println("x")
	if err != nil {
	}
	return nil
}
`,
		"app.ts": `export function run() {
  console.log("debug temporary")
  throw new Error("TODO not implemented")
}
export function empty() {}
`,
		"job.py": `def empty_job():
    pass

def work():
    print("debug temporary")
    raise Exception("not implemented")
`,
		"Button.tsx": `export function Button() {
  console.log("debug temporary")
  return <button>Save</button>
}
`,
		"styles.scss": `.button {
  color: red;
  // TODO remove temporary style
}
`,
		"service.php": `<?php
function empty_service() {}
`,
		"worker.rs": `fn empty_worker() {}
`,
		"kernel.zig": `fn emptyKernel() void {}
`,
		"Panel.vue": `<template><div>ok</div></template>
<script setup>
console.log("debug temporary")
</script>
`,
		"Dockerfile": `FROM scratch
# TODO replace temporary image
`,
		"deploy.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: temporary
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := NewScanner(2).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Summary.FilesScanned != len(files) {
		t.Fatalf("files scanned = %d, want %d", result.Summary.FilesScanned, len(files))
	}
	if result.Summary.TreeSitterParsed < 3 {
		t.Fatalf("tree-sitter parsed = %d, want at least Go, TSX, and YAML", result.Summary.TreeSitterParsed)
	}
	for _, language := range []string{"Go", "TypeScript", "Python", "SCSS", "PHP", "Rust", "Zig", "Vue", "Dockerfile", "YAML"} {
		if result.Summary.Languages[language] < 1 {
			t.Fatalf("language %s missing in summary: %+v", language, result.Summary.Languages)
		}
	}

	seen := map[string]bool{}
	for _, finding := range result.Findings {
		seen[finding.Rule] = true
	}
	for _, rule := range []string{domain.RuleEmptyFunction, domain.RuleIgnoredResult, domain.RuleDebugPrint, domain.RulePanicPlaceholder, domain.RuleSwallowedError} {
		if !seen[rule] {
			t.Fatalf("missing rule %s in findings: %+v", rule, result.Findings)
		}
	}
}

func TestScannerSkipsVendorDirectoryByEnryRules(t *testing.T) {
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(vendorDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "ignored.js"), []byte("console.log('debug temporary')"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.js"), []byte("console.log('debug temporary')"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.FilesScanned != 1 {
		t.Fatalf("files scanned = %d, want 1: %+v", result.Summary.FilesScanned, result.Summary)
	}
}

func TestScannerIncludesUnknownTextAndSkipsBinary(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "custom.frontend")
	if err := os.WriteFile(textPath, []byte("component debug temporary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(dir, "image.png")
	if err := os.WriteFile(binaryPath, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Languages[LanguageText] != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
	if result.Summary.FilesScanned != 1 {
		t.Fatalf("files scanned = %d, want 1", result.Summary.FilesScanned)
	}
}

func TestScannerScoresByDensity(t *testing.T) {
	findings := []domain.Finding{{Severity: domain.SeverityHigh}, {Severity: domain.SeverityHigh}}
	shortScore := domain.Score(findings, domain.MinimumScoredLines)
	longScore := domain.Score(findings, domain.MinimumScoredLines*10)
	if shortScore <= longScore {
		t.Fatalf("shortScore = %d, longScore = %d", shortScore, longScore)
	}
}

func TestScannerReportsTreeSitterParseErrors(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "broken.tsx")
	if err := os.WriteFile(badPath, []byte("export function Broken() { return <div>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.TreeSitterParsed != 1 {
		t.Fatalf("tree-sitter parsed = %d, want 1", result.Summary.TreeSitterParsed)
	}
	if result.Summary.TreeSitterFailures != 1 {
		t.Fatalf("tree-sitter failures = %d, want 1", result.Summary.TreeSitterFailures)
	}
	for _, finding := range result.Findings {
		if finding.Rule == domain.RuleParseError {
			return
		}
	}
	t.Fatalf("missing parse error finding: %+v", result.Findings)
}

func TestScannerFindsAITellCommentAndSwallowedError(t *testing.T) {
	dir := t.TempDir()
	body := `package main

// Ensure robust seamless handling
func Work() error {
	err := run()
	if err != nil {
	}
	return nil
}
func run() error { return nil }
`
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, finding := range result.Findings {
		seen[finding.Rule] = true
	}
	if !seen[domain.RuleAITellComment] {
		t.Fatalf("missing ai_tell_comment: %+v", result.Findings)
	}
	if !seen[domain.RuleSwallowedError] {
		t.Fatalf("missing swallowed_error: %+v", result.Findings)
	}
}
