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
	for _, language := range []string{"Go", "TypeScript", "Python", "Tsx", "Scss"} {
		if result.Summary.Languages[language] != 1 {
			t.Fatalf("language %s missing in summary: %+v", language, result.Summary.Languages)
		}
	}

	seen := map[string]bool{}
	for _, finding := range result.Findings {
		seen[finding.Rule] = true
	}
	for _, rule := range []string{domain.RuleEmptyFunction, domain.RuleIgnoredResult, domain.RuleDebugPrint, domain.RulePanicPlaceholder} {
		if !seen[rule] {
			t.Fatalf("missing rule %s in findings: %+v", rule, result.Findings)
		}
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
