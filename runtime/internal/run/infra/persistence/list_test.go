package persistence

import (
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
)

func TestListReturnsSlugsWithManifest(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"alpha", "beta"} {
		run, err := domain.NewRun(slug, "goal", "goal-delivery", 50, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		path, _ := indexedPaths(t, root, slug)
		if err := Save(path, run); err != nil {
			t.Fatal(err)
		}
	}
	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("List = %v", got)
	}
}

func TestListMissingRunsDirReturnsEmpty(t *testing.T) {
	got, err := List(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("List = %v, want empty", got)
	}
}
