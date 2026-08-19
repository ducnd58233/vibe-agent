package sessionread_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/sessionread"
)

func TestFSReplayAndPeekHost(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	logPath := session.LogPath(root, slug)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"session_start","payload":{"client":"cursor"},"event":"SessionStart"}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := sessionread.NewFS()
	events, err := reader.Replay(root, slug)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if got := reader.PeekHost(logPath); got != "cursor" {
		t.Fatalf("PeekHost = %q", got)
	}
}

func TestFSAmbientStat(t *testing.T) {
	root := t.TempDir()
	path := session.AmbientLogPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	stat := sessionread.NewFS().AmbientStat(root)
	if !stat.Present || stat.Size != 2 {
		t.Fatalf("stat = %+v", stat)
	}
	if stat.ModTime.IsZero() {
		t.Fatal("expected mod time")
	}
}
