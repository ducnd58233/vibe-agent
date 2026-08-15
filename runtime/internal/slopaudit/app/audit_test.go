package app

import (
	"context"
	"errors"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

type failingScanner struct{}

func (failingScanner) Scan(ctx context.Context, target string) ([]domain.Finding, error) {
	return nil, errors.New("target missing")
}

func TestAuditorReportsScannerFailure(t *testing.T) {
	auditor := NewAuditor([]Scanner{failingScanner{}}, nil)
	report := auditor.Audit(context.Background(), "missing")
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %+v", report.Findings)
	}
	if report.Findings[0].Rule != domain.RuleScanError {
		t.Fatalf("rule = %q, want %q", report.Findings[0].Rule, domain.RuleScanError)
	}
}
