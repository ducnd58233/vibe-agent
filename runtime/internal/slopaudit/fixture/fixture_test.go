package fixture

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit"
)

func TestSeededFixturesSeparateCleanAndSlop(t *testing.T) {
	root := filepath.Join("testdata")
	clean := slopaudit.Audit(context.Background(), filepath.Join(root, "clean"), slopaudit.Options{Workers: 2})
	slop := slopaudit.Audit(context.Background(), filepath.Join(root, "slop"), slopaudit.Options{Workers: 2})

	if clean.Summary.FilesScanned != 3 || slop.Summary.FilesScanned != 3 {
		t.Fatalf("unexpected fixture coverage: clean=%+v slop=%+v", clean.Summary, slop.Summary)
	}
	if slop.Score <= clean.Score {
		t.Fatalf("scores did not separate fixtures: clean=%d slop=%d", clean.Score, slop.Score)
	}
	if len(slop.Findings) <= len(clean.Findings) {
		t.Fatalf("finding counts did not separate fixtures: clean=%d slop=%d", len(clean.Findings), len(slop.Findings))
	}
}
