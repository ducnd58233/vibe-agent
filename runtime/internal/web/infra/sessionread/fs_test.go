package sessionread_test

import (
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/sessionread"
)

func TestFSReplayAndPeekHost(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	testutil.EnsureRunIndex(t, root, slug)
	logPath := session.LogPath(root, slug)
	if _, err := session.Append(logPath, session.Record{
		Type: session.TypeSessionStart, Source: session.SourceHook, Client: "cursor", Event: "SessionStart",
	}); err != nil {
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
	if stat := sessionread.NewFS().AmbientStat(root); stat.Present {
		t.Fatalf("empty workspace reported an ambient journal: %+v", stat)
	}
	when := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	if _, err := session.Append(session.AmbientLogPath(root), session.Record{
		Type: session.TypePromptSubmit, Source: session.SourceHook, Body: "hello", At: when,
	}); err != nil {
		t.Fatal(err)
	}
	stat := sessionread.NewFS().AmbientStat(root)
	if !stat.Present || stat.Size != 1 {
		t.Fatalf("stat = %+v", stat)
	}
	if !stat.ModTime.Equal(when) {
		t.Fatalf("ModTime = %v, want %v", stat.ModTime, when)
	}
}
