package fixture

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit"
)

func BenchmarkAuditCleanFixture(b *testing.B) {
	target := filepath.Join("testdata", "clean")
	ctx := context.Background()
	opts := slopaudit.Options{Workers: 2}
	b.ReportAllocs()
	for b.Loop() {
		report := slopaudit.Audit(ctx, target, opts)
		if report.Summary.FilesScanned != 3 {
			b.Fatalf("files scanned = %d, want 3", report.Summary.FilesScanned)
		}
	}
}

func BenchmarkAuditSlopFixture(b *testing.B) {
	target := filepath.Join("testdata", "slop")
	ctx := context.Background()
	opts := slopaudit.Options{Workers: 2}
	b.ReportAllocs()
	for b.Loop() {
		report := slopaudit.Audit(ctx, target, opts)
		if report.Summary.FilesScanned != 3 {
			b.Fatalf("files scanned = %d, want 3", report.Summary.FilesScanned)
		}
	}
}
