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

func TestAProposedMemoryKeepsItsAuthor(t *testing.T) {
	store, err := OpenAt(t.Context(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

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
	path := filepath.Join(t.TempDir(), "memory.db")
	old, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := old.ExecContext(t.Context(), `
        CREATE TABLE memories (
            id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, kind TEXT NOT NULL,
            content TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '',
            confidence REAL NOT NULL, status TEXT NOT NULL,
            source_type TEXT NOT NULL, source_ref TEXT, evidence TEXT NOT NULL,
            supersedes_id TEXT, used_count INTEGER NOT NULL DEFAULT 0,
            expires_at TEXT, valid_from TEXT NOT NULL DEFAULT '', valid_to TEXT,
            created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
        CREATE VIRTUAL TABLE memories_fts USING fts5(memory_id UNINDEXED, content, tags);
        INSERT INTO memories (id, workspace_id, kind, content, confidence, status,
            source_type, evidence, valid_from, created_at, updated_at)
        VALUES ('mem_legacy', 'ws', 'semantic', 'the build runs on node 20', 0.9,
            'confirmed', 'command_result', 'node --version printed v20.11.0',
            '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z');`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = old.Close()

	store, err := OpenAt(t.Context(), path)
	if err != nil {
		t.Fatalf("open a pre-provenance database: %v", err)
	}
	defer func() { _ = store.Close() }()
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
	if WorkspaceKey(`D:\projects\old-home`) != WorkspaceKey("/home/someone/new-home") {
		t.Fatal("the same workspace gets a different key from a different path")
	}
	path := filepath.Join(t.TempDir(), "memory.db")
	old, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := old.ExecContext(t.Context(), `
        CREATE TABLE memories (
            id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, kind TEXT NOT NULL,
            content TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '',
            confidence REAL NOT NULL, status TEXT NOT NULL,
            source_type TEXT NOT NULL, source_ref TEXT, evidence TEXT NOT NULL,
            supersedes_id TEXT, used_count INTEGER NOT NULL DEFAULT 0,
            expires_at TEXT, valid_from TEXT NOT NULL DEFAULT '', valid_to TEXT,
            created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
        CREATE VIRTUAL TABLE memories_fts USING fts5(memory_id UNINDEXED, content, tags);
        INSERT INTO memories (id, workspace_id, kind, content, confidence, status,
            source_type, evidence, valid_from, created_at, updated_at)
        VALUES ('mem_win', 'D:\projects\old-home', 'semantic', 'a', 0.9, 'confirmed',
            'command_result', 'e', '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z'),
               ('mem_posix', '/d/projects/old-home', 'semantic', 'b', 0.9, 'confirmed',
            'command_result', 'e', '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z');`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = old.Close()

	store, err := OpenAt(t.Context(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()
	records, err := store.List(t.Context(), WorkspaceKey("wherever/it/lives/now"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("found %d of 2 memories written under absolute keys", len(records))
	}
}
