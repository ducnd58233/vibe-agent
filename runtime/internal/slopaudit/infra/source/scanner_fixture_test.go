package source

import (
	"path/filepath"
	"testing"
)

func TestSourceFilesFromBenchmarkRelativePath(t *testing.T) {
	benchmarkDir, err := filepath.Abs(filepath.Join("..", "..", "benchmark"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(benchmarkDir)
	files, err := sourceFiles("testdata/clean")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files = %d, want 3: %v", len(files), files)
	}
}
