package app

import (
	"context"
	"sync"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

const DefaultWorkers = 4

type Scanner interface {
	Scan(ctx context.Context, target string) (domain.ScanResult, error)
}

type Adapter interface {
	Run(ctx context.Context, target string) domain.AdapterResult
}

type Auditor struct {
	scanners []Scanner
	adapters []Adapter
}

func NewAuditor(scanners []Scanner, adapters []Adapter) *Auditor {
	return &Auditor{scanners: scanners, adapters: adapters}
}

func (a *Auditor) Audit(ctx context.Context, target string) domain.Report {
	scanResult := a.scan(ctx, target)
	adapters := a.runAdapters(ctx, target)
	domain.SortFindings(scanResult.Findings)
	domain.SortAdapters(adapters)
	score := domain.Score(scanResult.Findings, scanResult.Summary.LinesScanned)
	return domain.Report{
		Target:   target,
		Score:    score,
		Status:   domain.Status(score),
		Summary:  scanResult.Summary,
		Findings: scanResult.Findings,
		Adapters: adapters,
	}
}

func (a *Auditor) scan(ctx context.Context, target string) domain.ScanResult {
	var wg sync.WaitGroup
	out := make(chan domain.ScanResult, len(a.scanners))
	for _, scanner := range a.scanners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := scanner.Scan(ctx, target)
			if err != nil {
				out <- domain.ScanResult{Findings: []domain.Finding{scanErrorFinding(target, err)}}
				return
			}
			out <- result
		}()
	}
	wg.Wait()
	close(out)

	var findings []domain.Finding
	var summaries []domain.ScanSummary
	for result := range out {
		findings = append(findings, result.Findings...)
		summaries = append(summaries, result.Summary)
	}
	return domain.ScanResult{Findings: findings, Summary: domain.MergeSummary(summaries)}
}

func scanErrorFinding(target string, err error) domain.Finding {
	return domain.Finding{
		Path:     target,
		Line:     1,
		Rule:     domain.RuleScanError,
		Severity: domain.SeverityHigh,
		Message:  err.Error(),
	}
}

func (a *Auditor) runAdapters(ctx context.Context, target string) []domain.AdapterResult {
	var wg sync.WaitGroup
	out := make(chan domain.AdapterResult, len(a.adapters))
	for _, adapter := range a.adapters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out <- adapter.Run(ctx, target)
		}()
	}
	wg.Wait()
	close(out)

	var results []domain.AdapterResult
	for result := range out {
		results = append(results, result)
	}
	return results
}
