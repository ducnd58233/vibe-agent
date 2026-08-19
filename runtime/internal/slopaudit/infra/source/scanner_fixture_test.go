package source

import (
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestSourceFilesFromFixtureTree(t *testing.T) {
	root := filepath.Join(testutil.RuntimeRoot(t), "internal", "slopaudit", "fixture", "testdata", "clean")
	files, err := sourceFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files = %d, want 3: %v", len(files), files)
	}
}
