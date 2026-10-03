package memory

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func openFileStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.db")
	store, err := OpenAt(t.Context(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path
}

// The ledger is what survives in-place edits, so every status change must leave
// a line, in order, naming who did it.
func TestLedgerRecordsEveryStatusChangeInOrder(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)

	record, _, err := store.Propose(ctx, candidate(func(r *Record) {
		r.CreatedBy = "codex/gpt-5"
	}), day(1))
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if _, _, err := store.Propose(ctx, candidate(func(r *Record) {
		r.Evidence = []string{"a second, independent observation of the same failure"}
	}), day(2)); err != nil {
		t.Fatalf("propose duplicate: %v", err)
	}
	if err := store.AddReviewer(ctx, record.ID, "claude", day(3)); err != nil {
		t.Fatalf("review: %v", err)
	}
	if _, err := store.Confirm(ctx, record.ID, SourceHumanStatement, "terminal", day(4)); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := store.Invalidate(ctx, record.ID, day(5)); err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	history, err := store.History(ctx, record.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	var actions []string
	for _, entry := range history {
		actions = append(actions, entry.Action)
	}
	if got := strings.Join(actions, ","); got != "propose,merge,review,confirm,invalidate" {
		t.Fatalf("actions = %s", got)
	}
	if history[0].Actor != "codex/gpt-5" || history[0].ToStatus != "proposed" {
		t.Errorf("propose entry = %+v", history[0])
	}
	confirm := history[3]
	if confirm.FromStatus != "proposed" || confirm.ToStatus != "confirmed" || confirm.Actor != string(SourceHumanStatement) {
		t.Errorf("confirm entry = %+v", confirm)
	}
	closed := history[4]
	if closed.FromStatus != "confirmed" || closed.ToStatus != "stale" {
		t.Errorf("invalidate entry = %+v", closed)
	}
}

func TestSupersedingMemoryIsLinkedAutomatically(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	original := confirmMemory(t, store, "the api listens on port 8080", day(1))
	replacement, _, err := store.Propose(ctx, Record{
		WorkspaceID: "ws", Kind: KindSemantic, Content: "the api listens on port 9090",
		Confidence: 0.9, SourceType: SourceFileContent, SupersedesID: original.ID,
		Evidence: []string{"config/server.yaml sets port: 9090"},
	}, day(2))
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	links, err := store.Links(ctx, original.ID)
	if err != nil {
		t.Fatalf("links: %v", err)
	}
	if len(links) != 1 || links[0].SrcID != replacement.ID || links[0].Relation != RelationSupersedes {
		t.Fatalf("links = %+v", links)
	}
}

func TestLinkRefusesBadEdges(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	a := confirmMemory(t, store, "the build uses make", day(1))
	b := confirmMemory(t, store, "the linter is golangci-lint", day(2))

	for name, link := range map[string]Link{
		"unknown relation": {SrcID: a.ID, DstID: b.ID, Relation: "similar_to"},
		"self link":        {SrcID: a.ID, DstID: a.ID, Relation: RelationRelatesTo},
		"missing target":   {SrcID: a.ID, DstID: "mem_nope", Relation: RelationRelatesTo},
	} {
		if err := store.Link(ctx, link, day(3)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}

	good := Link{SrcID: a.ID, DstID: b.ID, Relation: RelationRelatesTo}
	for i := 0; i < 2; i++ {
		if err := store.Link(ctx, good, day(3)); err != nil {
			t.Fatalf("link %d: %v", i, err)
		}
	}
	history, _ := store.History(ctx, a.ID)
	linkLines := 0
	for _, entry := range history {
		if entry.Action == "link" {
			linkLines++
		}
	}
	if linkLines != 1 {
		t.Errorf("re-adding an edge wrote %d ledger lines, want 1", linkLines)
	}
}

func TestSearchPullsInLinkedNeighboursOnlyIntoSpareSlots(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	hit := confirmMemory(t, store, "deploys run through the release pipeline", day(1))
	neighbour := confirmMemory(t, store, "the staging cluster is named orca", day(2))
	stranger := confirmMemory(t, store, "unrelated note about lunch", day(3))
	if err := store.Link(ctx, Link{SrcID: hit.ID, DstID: neighbour.ID, Relation: RelationRelatesTo}, day(4)); err != nil {
		t.Fatalf("link: %v", err)
	}

	hits, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "release pipeline"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 || hits[0].ID != hit.ID || hits[1].ID != neighbour.ID {
		t.Fatalf("hits = %v", contents(hits))
	}
	if hits[0].Via != "" || !strings.Contains(hits[1].Via, hit.ID) {
		t.Errorf("via = %q / %q", hits[0].Via, hits[1].Via)
	}
	for _, h := range hits {
		if h.ID == stranger.ID {
			t.Error("an unlinked memory was returned")
		}
	}

	// A full result list leaves no room: neighbours never displace a match.
	capped, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "release pipeline", Limit: 1})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(capped) != 1 || capped[0].ID != hit.ID {
		t.Errorf("capped = %v", contents(capped))
	}

	// A closed neighbour is not resurrected by the edge.
	if err := store.Invalidate(ctx, neighbour.ID, day(5)); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	after, _ := store.Search(ctx, Query{WorkspaceID: "ws", Text: "release pipeline"})
	if len(after) != 1 {
		t.Errorf("a stale neighbour was returned: %v", contents(after))
	}
}

func TestContradictsEdgeIsNotFollowed(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	a := confirmMemory(t, store, "retries are capped at three", day(1))
	b := confirmMemory(t, store, "retries are unbounded", day(2))
	if err := store.Link(ctx, Link{SrcID: a.ID, DstID: b.ID, Relation: RelationContradicts}, day(3)); err != nil {
		t.Fatalf("link: %v", err)
	}
	hits, _ := store.Search(ctx, Query{WorkspaceID: "ws", Text: "capped"})
	if len(hits) != 1 {
		t.Errorf("a contradicting memory was pulled in: %v", contents(hits))
	}
}

// Importance breaks what recency and keyword rank cannot: two equally relevant,
// equally fresh memories, one of which reuse has repeatedly confirmed.
func TestSearchPrefersTheMemoryReuseHasConfirmed(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	older := confirmMemory(t, store, "cache invalidation uses the version header", day(1))
	newer := confirmMemory(t, store, "cache invalidation relies on version header values", day(1))
	for i := 0; i < 6; i++ {
		if err := store.RecordUse(ctx, older.ID, day(2)); err != nil {
			t.Fatalf("use: %v", err)
		}
	}
	hits, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "cache invalidation version header"})
	if err != nil || len(hits) != 2 {
		t.Fatalf("search: %v %v", err, contents(hits))
	}
	if hits[0].ID != older.ID {
		t.Errorf("reused memory ranked second: %v (newer=%s)", contents(hits), newer.ID)
	}
}

func TestStemmingMatchesInflectedWords(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	confirmMemory(t, store, "the integration suite failed when redis was down", day(1))
	hits, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "failing"})
	if err != nil || len(hits) != 1 {
		t.Fatalf("stemmed search: %v %v", err, contents(hits))
	}
}

func TestSearchSessionsFindsTextAndSkipsThinking(t *testing.T) {
	ctx := t.Context()
	store, path := openFileStore(t)
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	for i, row := range []struct{ typ, payload string }{
		{"prompt_submit", `{"body":"why does the webhook retry loop never stop"}`},
		{"thinking", `{"body":"private webhook reasoning"}`},
		{"tool_use", `{"tool":"bash","command":"go test ./webhook/..."}`},
		{"message", `not json at all`},
	} {
		if _, err := raw.ExecContext(ctx, `INSERT INTO session_events (scope, sequence, type, at, payload, created_at)
            VALUES ('ambient', ?, ?, '2026-08-01T10:00:00Z', ?, '2026-08-01T10:00:00Z')`, i+1, row.typ, row.payload); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	hits, err := store.SearchSessions(ctx, "webhooks", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %+v", hits)
	}
	for _, hit := range hits {
		if hit.Type == "thinking" {
			t.Error("host reasoning was indexed")
		}
		if hit.Scope != "ambient" || hit.Snippet == "" {
			t.Errorf("hit = %+v", hit)
		}
	}
	if _, err := store.SearchSessions(ctx, "  ", 5); err == nil {
		t.Error("an empty query was accepted")
	}
}
