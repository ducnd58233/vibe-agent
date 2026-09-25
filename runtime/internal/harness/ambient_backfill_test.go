package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func writeLegacyAmbientJournal(t *testing.T, path string, events []state.Event) {
	t.Helper()
	var raw []byte
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, line...)
		raw = append(raw, '\n')
	}
	if err := os.WriteFile(filepath.Clean(path), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAmbientJournalBackfillMovesEveryExistingLineThenDeletesTheFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(workspace.StateDir(root), 0o750); err != nil {
		t.Fatal(err)
	}
	path := legacyAmbientJournalPath(root)

	events := []state.Event{
		{Sequence: 1, Type: state.EventToolUse, Payload: json.RawMessage(`{"tool":"Bash","command":"ls"}`),
			At: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
		{Sequence: 2, Type: state.EventToolUse, Payload: json.RawMessage(`{"tool":"Edit","file":"x.go"}`),
			At: time.Date(2026, 9, 1, 0, 1, 0, 0, time.UTC)},
	}
	writeLegacyAmbientJournal(t, path, events)

	migrated, err := AmbientJournalBackfill(context.Background(), root)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if migrated != len(events) {
		t.Errorf("migrated %d entries, want %d", migrated, len(events))
	}

	ctx := context.Background()
	db, err := database.Open(ctx, workspace.MemoryDBPath(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM journal_entries WHERE run_id IS NULL`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(events) {
		t.Errorf("journal_entries has %d ambient rows, want %d", count, len(events))
	}
	var payload string
	if err := db.QueryRowContext(ctx, `SELECT payload FROM journal_entries WHERE id = 1`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if payload != string(events[0].Payload) {
		t.Errorf("row 1 payload = %q, want %q", payload, events[0].Payload)
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("the legacy journal file still exists after backfill")
	}
}

func TestAmbientJournalBackfillIsSafeWithNothingToMove(t *testing.T) {
	root := t.TempDir()
	migrated, err := AmbientJournalBackfill(context.Background(), root)
	if err != nil {
		t.Fatalf("backfill on an empty workspace: %v", err)
	}
	if migrated != 0 {
		t.Errorf("migrated %d entries from an empty workspace, want 0", migrated)
	}
}

// A corrupted legacy file stops the backfill rather than losing lines: the
// file is one append-only log, not one file per entry, so there is no
// "everything before the bad line" to salvage without risking a line counted
// twice on a retry.
func TestAmbientJournalBackfillStopsOnACorruptedFileRatherThanLosingLines(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(workspace.StateDir(root), 0o750); err != nil {
		t.Fatal(err)
	}
	path := legacyAmbientJournalPath(root)
	if err := os.WriteFile(path, []byte("{not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	migrated, err := AmbientJournalBackfill(context.Background(), root)
	if err == nil {
		t.Fatal("a corrupted legacy journal did not stop the backfill")
	}
	if migrated != 0 {
		t.Errorf("migrated = %d, want 0", migrated)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Error("the corrupted file was removed despite failing to migrate")
	}
}
