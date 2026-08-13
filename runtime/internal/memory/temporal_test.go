package memory

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func day(n int) time.Time {
	return time.Date(2026, 8, n, 12, 0, 0, 0, time.UTC)
}

func storeWithMemory(t *testing.T, content string, at time.Time) (*Store, Record) {
	t.Helper()
	store, err := OpenAt(t.Context(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	record := confirmMemory(t, store, content, at)
	return store, record
}

func confirmMemory(t *testing.T, store *Store, content string, at time.Time) Record {
	t.Helper()
	ctx := t.Context()
	stored, decision, err := store.Propose(ctx, Record{
		WorkspaceID: "ws",
		Kind:        KindSemantic,
		Content:     content,
		Confidence:  0.8,
		SourceType:  SourceCommandResult,
		Evidence:    []string{"observed by running the suite against this workspace"},
	}, at)
	if err != nil {
		t.Fatalf("propose %q: %v", content, err)
	}
	if decision.Verdict == VerdictReject {
		t.Fatalf("propose %q rejected: %s", content, decision.Reason)
	}
	confirmed, err := store.Confirm(ctx, stored.ID, SourceCommandResult, "events.ndjson#1", at)
	if err != nil {
		t.Fatalf("confirm %q: %v", content, err)
	}
	return confirmed
}

func contents(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Content)
	}
	return out
}

// The riskiest path in this change is the one no new workspace takes: a
// database written before the validity interval existed. CREATE TABLE IF NOT
// EXISTS does nothing to it, so without the migration every query would fail on
// a missing column.
func TestADatabaseFromBeforeTheIntervalStillOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")

	old, err := sql.Open("sqlite", path)
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
            expires_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
        CREATE VIRTUAL TABLE memories_fts USING fts5(memory_id UNINDEXED, content, tags);
        INSERT INTO memories (id, workspace_id, kind, content, confidence, status,
            source_type, evidence, created_at, updated_at)
        VALUES ('mem_legacy', 'ws', 'semantic', 'the build runs on node 20', 0.9,
            'confirmed', 'command_result', 'node --version printed v20.11.0',
            '2026-07-01T09:00:00Z', '2026-07-01T09:00:00Z');
        INSERT INTO memories_fts (memory_id, content, tags)
        VALUES ('mem_legacy', 'the build runs on node 20', '');`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	_ = old.Close()

	store, err := OpenAt(t.Context(), path)
	if err != nil {
		t.Fatalf("opening a database from an earlier version failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	hits, err := store.Search(t.Context(), Query{WorkspaceID: "ws"})
	if err != nil {
		t.Fatalf("search a migrated database: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("the migrated memory was lost: %v", contents(hits))
	}
	// Backfilled from created_at: the closest honest answer available for a row
	// recorded before anyone was tracking when its fact started.
	if !hits[0].ValidFrom.Equal(time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("validFrom was not backfilled from created_at: %v", hits[0].ValidFrom)
	}
}

func TestAConfirmedMemoryIsValidFromWhenItWasRecorded(t *testing.T) {
	_, record := storeWithMemory(t, "the api listens on port 8080", day(1))
	if !record.ValidFrom.Equal(day(1)) {
		t.Errorf("validFrom is %v, want %v", record.ValidFrom, day(1))
	}
	if record.ValidTo != nil {
		t.Errorf("a live memory has an end date: %v", record.ValidTo)
	}
}

// Graphiti's rule, reduced to what this store needs: a fact that stops being
// true is closed, not deleted, so the record of having believed it survives.
func TestInvalidateClosesAMemoryInsteadOfDeletingIt(t *testing.T) {
	store, record := storeWithMemory(t, "the api listens on port 8080", day(1))
	ctx := t.Context()

	if err := store.Invalidate(ctx, record.ID, day(5)); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	closed, err := store.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("the record was deleted rather than closed: %v", err)
	}
	if closed.ValidTo == nil || !closed.ValidTo.Equal(day(5)) {
		t.Errorf("validTo is %v, want %v", closed.ValidTo, day(5))
	}
	if closed.Status != StatusStale {
		t.Errorf("status is %s, want stale", closed.Status)
	}
}

func TestSearchSkipsAnInvalidatedMemory(t *testing.T) {
	store, record := storeWithMemory(t, "the api listens on port 8080", day(1))
	ctx := t.Context()
	if err := store.Invalidate(ctx, record.ID, day(5)); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	hits, err := store.Search(ctx, Query{WorkspaceID: "ws"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("retrieval returned a closed memory: %v", contents(hits))
	}
}

// The point of keeping the interval rather than a flag: the store can still
// answer what it believed at a past moment.
func TestSearchAsOfReturnsWhatWasBelievedThen(t *testing.T) {
	store, record := storeWithMemory(t, "the api listens on port 8080", day(1))
	ctx := t.Context()
	if err := store.Invalidate(ctx, record.ID, day(5)); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	before, err := store.Search(ctx, Query{WorkspaceID: "ws", AsOf: day(3)})
	if err != nil {
		t.Fatalf("search as of: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("as-of retrieval lost the fact that was true then: %v", contents(before))
	}

	after, err := store.Search(ctx, Query{WorkspaceID: "ws", AsOf: day(7)})
	if err != nil {
		t.Fatalf("search as of: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("as-of retrieval returned a fact that had already ended: %v", contents(after))
	}
}

// Confirming a replacement closes the thing it replaces at the moment the
// replacement became true, rather than leaving both sides of a contradiction
// retrievable.
func TestConfirmingASupersedingMemoryClosesTheOldOne(t *testing.T) {
	store, original := storeWithMemory(t, "the api listens on port 8080", day(1))
	ctx := t.Context()

	replacement, decision, err := store.Propose(ctx, Record{
		WorkspaceID:  "ws",
		Kind:         KindSemantic,
		Content:      "the api listens on port 9090",
		Confidence:   0.9,
		SourceType:   SourceFileContent,
		Evidence:     []string{"config/server.yaml sets the listen port to 9090"},
		SupersedesID: original.ID,
	}, day(4))
	if err != nil || decision.Verdict == VerdictReject {
		t.Fatalf("propose replacement: %v %s", err, decision.Reason)
	}
	if _, err := store.Confirm(ctx, replacement.ID, SourceFileContent, "config/server.yaml", day(4)); err != nil {
		t.Fatalf("confirm replacement: %v", err)
	}

	closed, err := store.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get original: %v", err)
	}
	if closed.ValidTo == nil || !closed.ValidTo.Equal(day(4)) {
		t.Errorf("the superseded fact closed at %v, want %v", closed.ValidTo, day(4))
	}

	hits, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "api listens port"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Content != "the api listens on port 9090" {
		t.Errorf("retrieval returned both sides of a contradiction: %v", contents(hits))
	}
}

// --- hybrid retrieval --------------------------------------------------------

// Keyword rank alone puts a year-old match above a fresh one that matches
// slightly less well. Fusing the two ranked lists is what fixes that, and it
// needs no weights to tune.
func TestSearchFusesKeywordRankWithRecency(t *testing.T) {
	store, err := OpenAt(t.Context(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()

	confirmMemory(t, store, "the payment service retries webhook delivery", day(1))
	confirmMemory(t, store, "the payment service now retries webhook delivery with backoff", day(20))

	hits, err := store.Search(t.Context(), Query{
		WorkspaceID: "ws", Text: "payment webhook retries",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want both matches, got %v", contents(hits))
	}
	if hits[0].Content != "the payment service now retries webhook delivery with backoff" {
		t.Errorf("the newer fact did not rank first: %v", contents(hits))
	}
}

func TestSearchStillHonoursTheLimitAfterFusing(t *testing.T) {
	store, err := OpenAt(t.Context(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()

	for _, content := range []string{
		"the queue consumer commits offsets after processing",
		"the queue consumer batches offsets every second",
		"the queue consumer retries on a poison message",
	} {
		confirmMemory(t, store, content, day(1))
	}

	hits, err := store.Search(t.Context(), Query{
		WorkspaceID: "ws", Text: "queue consumer offsets", Limit: 2,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("limit was not applied after fusing: got %d", len(hits))
	}
}
