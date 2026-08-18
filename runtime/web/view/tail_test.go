package view

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

func TestEventsAfterReturnsNewRowsOnly(t *testing.T) {
	root := t.TempDir()
	slug := "tail-test"
	if err := os.MkdirAll(filepath.Join(root, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	path := session.LogPath(root, slug)
	stamp := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	first, err := session.Append(path, session.Record{
		Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "claude", Body: "first", At: stamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := EventsAfter(root, slug, 0)
	if err != nil || len(all) != 1 {
		t.Fatalf("all rows = %d err = %v", len(all), err)
	}
	none, err := EventsAfter(root, slug, first.Sequence)
	if err != nil || len(none) != 0 {
		t.Fatalf("after first = %d err = %v", len(none), err)
	}
	second, err := session.Append(path, session.Record{
		Type: session.TypePromptSubmit, Source: session.SourceHook, Client: "claude", Body: "second", At: stamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	tail, err := EventsAfter(root, slug, first.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 1 || tail[0].Seq != second.Sequence {
		t.Fatalf("tail = %+v want seq %d", tail, second.Sequence)
	}
}

func TestLastSequenceEmptyLog(t *testing.T) {
	root := t.TempDir()
	if LastSequence(root, "missing") != 0 {
		t.Fatal("expected zero for missing log")
	}
}
