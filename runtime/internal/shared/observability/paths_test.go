package observability

import (
	"path/filepath"
	"testing"
)

func TestLogDirFromExecutableSiblingOfBin(t *testing.T) {
	root := t.TempDir()
	exec := filepath.Join(root, "bin", "vibe-agent")
	got := logDirFromExecutable(exec)
	want := filepath.Join(root, "logs")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLogDirFromExecutableSameDirectory(t *testing.T) {
	root := t.TempDir()
	exec := filepath.Join(root, "vibe-agent")
	got := logDirFromExecutable(exec)
	want := filepath.Join(root, "logs")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
