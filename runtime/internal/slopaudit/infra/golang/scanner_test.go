package golang

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

func TestScannerFindsGoSlopSignals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	code := `package sample

import "fmt"

// TODO wire this for real.
func Empty() {}

func Work() {
	_ = risky()
	fmt.Println("debug")
	panic("TODO not implemented")
}

func risky() error { return nil }
`
	if err := os.WriteFile(path, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(2).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, finding := range result.Findings {
		seen[finding.Rule] = true
	}
	for _, rule := range []string{domain.RuleTodoComment, domain.RuleEmptyFunction, domain.RuleIgnoredResult, domain.RuleDebugPrint, domain.RulePanicPlaceholder} {
		if !seen[rule] {
			t.Fatalf("missing rule %s in findings: %+v", rule, result.Findings)
		}
	}
	if result.Summary.Languages["Go"] != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
}

func TestScoreIsCapped(t *testing.T) {
	var findings []domain.Finding
	for i := 0; i < 20; i++ {
		findings = append(findings, domain.Finding{Severity: domain.SeverityHigh})
	}
	if got := domain.Score(findings, domain.MinimumScoredLines); got != domain.MaxScore {
		t.Fatalf("score = %d, want %d", got, domain.MaxScore)
	}
}
