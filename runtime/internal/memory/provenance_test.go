package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
)

// Several harnesses can share one workspace. A memory that cannot say which
// agent wrote it, or which agents checked it, cannot be weighed or audited by
// the others.

// openStoreAt opens a store and closes it when the test ends, failing the test
// if closing fails.
func openStoreAt(t *testing.T, path string) *Store {
	t.Helper()
	store, err := OpenAt(t.Context(), path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return store
}

// legacySchema is the memories table as it stood before provenance, after the
// validity interval.
const legacySchema = `
    CREATE TABLE memories (
        id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, kind TEXT NOT NULL,
        content TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '',
        confidence REAL NOT NULL, status TEXT NOT NULL,
        source_type TEXT NOT NULL, source_ref TEXT, evidence TEXT NOT NULL,
        supersedes_id TEXT, used_count INTEGER NOT NULL DEFAULT 0,
        expires_at TEXT, valid_from TEXT NOT NULL DEFAULT '', valid_to TEXT,
        created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
    CREATE VIRTUAL TABLE memories_fts USING fts5(memory_id UNINDEXED, content, tags);`

// openLegacy writes one row per (id, workspace key) into a pre-provenance
// database, then opens it with the current code, which migrates it.
func openLegacy(t *testing.T, rows map[string]string) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.db")
	old, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := old.ExecContext(t.Context(), legacySchema); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	for id, workspace := range rows {
		if _, err := old.ExecContext(t.Context(), `
            INSERT INTO memories (id, workspace_id, kind, content, confidence, status,
                source_type, evidence, valid_from, created_at, updated_at)
            VALUES (?, ?, 'semantic', 'the build runs on node 20', 0.9, 'confirmed',
                'command_result', 'node --version printed v20.11.0',
                '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z')`,
			id, workspace); err != nil {
			t.Fatalf("seed row %s: %v", id, err)
		}
	}
	if err := old.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}
	return openStoreAt(t, path)
}

func TestAProposedMemoryKeepsItsAuthor(t *testing.T) {
	store := openStoreAt(t, ":memory:")
	stored, decision, err := store.Propose(t.Context(), Record{
		WorkspaceID: "ws", Kind: KindSemantic, Content: "the api listens on port 8080",
		Confidence: 0.8, SourceType: SourceCommandResult,
		Evidence:  []string{"curl localhost:8080 answered"},
		CreatedBy: "codex/gpt-5",
	}, day(1))
	if err != nil || decision.Verdict == VerdictReject {
		t.Fatalf("propose: %v %s", err, decision.Reason)
	}
	got, err := store.Get(t.Context(), stored.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CreatedBy != "codex/gpt-5" {
		t.Errorf("CreatedBy = %q, want codex/gpt-5", got.CreatedBy)
	}
	if len(got.ReviewedBy) != 0 {
		t.Errorf("a fresh memory already lists reviewers: %v", got.ReviewedBy)
	}
}

func TestAddReviewerRecordsEachAgentOnce(t *testing.T) {
	store, record := storeWithMemory(t, "the api listens on port 8080", day(1))
	ctx := t.Context()

	for _, agent := range []string{"claude", "cursor", "claude"} {
		if err := store.AddReviewer(ctx, record.ID, agent, day(2)); err != nil {
			t.Fatalf("AddReviewer(%s): %v", agent, err)
		}
	}
	got, err := store.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if strings.Join(got.ReviewedBy, ",") != "claude,cursor" {
		t.Errorf("ReviewedBy = %v, want claude then cursor, once each", got.ReviewedBy)
	}
	if !got.UpdatedAt.Equal(day(2)) {
		t.Errorf("UpdatedAt = %v, want the review time", got.UpdatedAt)
	}
	if got.Status != record.Status {
		t.Errorf("a review changed the status from %s to %s", record.Status, got.Status)
	}
}

func TestAddReviewerRefusesAnEmptyNameAndAMissingMemory(t *testing.T) {
	store, record := storeWithMemory(t, "the api listens on port 8080", day(1))
	if err := store.AddReviewer(t.Context(), record.ID, "  ", day(2)); err == nil {
		t.Error("an empty reviewer name was accepted")
	}
	if err := store.AddReviewer(t.Context(), "mem_missing", "claude", day(2)); err == nil {
		t.Error("a review of a memory that does not exist was accepted")
	}
}

// A database written before these columns existed must open and keep its rows,
// with an empty author rather than an invented one.
func TestADatabaseFromBeforeProvenanceStillOpens(t *testing.T) {
	store := openLegacy(t, map[string]string{"mem_legacy": "ws"})
	got, err := store.Get(t.Context(), "mem_legacy")
	if err != nil {
		t.Fatalf("the legacy memory was lost: %v", err)
	}
	if got.CreatedBy != "" || len(got.ReviewedBy) != 0 {
		t.Errorf("legacy row gained invented provenance: %q %v", got.CreatedBy, got.ReviewedBy)
	}
	if err := store.AddReviewer(t.Context(), "mem_legacy", "opencode", day(3)); err != nil {
		t.Errorf("a migrated row cannot be reviewed: %v", err)
	}
}

// Memories used to be keyed by the absolute workspace path. Moving the
// checkout, or opening it through another path form (D:\x versus /d/x), left
// every row unreachable. The database already lives inside its workspace, so
// the key is a portable constant and older absolute keys are rewritten.
func TestMemoriesKeyedByAnAbsolutePathSurviveAMove(t *testing.T) {
	if WorkspaceKey(`E:\work\old-home`) != WorkspaceKey("/srv/checkouts/new-home") {
		t.Fatal("the same workspace gets a different key from a different path")
	}
	store := openLegacy(t, map[string]string{
		"mem_win":   `E:\work\old-home`,
		"mem_posix": "/mnt/e/work/old-home",
		"mem_named": "ws",
	})
	records, err := store.List(t.Context(), WorkspaceKey("wherever/it/lives/now"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("found %d of the 2 memories written under absolute keys", len(records))
	}
	// A key that is not a path is a deliberate one, and the migration leaves it.
	named, err := store.List(t.Context(), "ws")
	if err != nil || len(named) != 1 {
		t.Errorf("a non-path key was rewritten: %d rows under ws, err %v", len(named), err)
	}
}
