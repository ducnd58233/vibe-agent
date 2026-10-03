package memory

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestSearchExcludesMemoriesAlreadyDelivered(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	first := confirmMemory(t, store, "deploys run through the release pipeline", day(1))
	second := confirmMemory(t, store, "the release pipeline signs artifacts with cosign", day(2))

	hits, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "release pipeline", Exclude: []string{first.ID}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != second.ID {
		t.Errorf("hits = %v, want only the memory not yet delivered", contents(hits))
	}
}

// Exclusion happens before the limit, so the freed slot goes to the next match
// instead of the result simply getting shorter.
func TestExcludedSlotsAreRefilled(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	var ids []string
	for i, text := range []string{
		"cache layer one handles session lookups",
		"cache layer two handles token lookups",
		"cache layer three handles profile lookups",
	} {
		ids = append(ids, confirmMemory(t, store, text, day(i+1)).ID)
	}
	hits, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "cache layer", Limit: 2, Exclude: ids[:1]})
	if err != nil || len(hits) != 2 {
		t.Fatalf("search: %v %v", err, contents(hits))
	}
	for _, hit := range hits {
		if hit.ID == ids[0] {
			t.Error("an excluded memory was returned")
		}
	}
}

func TestTokenBudgetKeepsRankOrderAndAlwaysTheTopHit(t *testing.T) {
	long := strings.Repeat("the build system caches compiled objects between runs ", 8)
	hits := []Hit{
		{Record: Record{ID: "a", Content: "short fact one"}},
		{Record: Record{ID: "b", Content: long}},
		{Record: Record{ID: "c", Content: "short fact three"}},
	}
	got := FitBudget(hits, 30)
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("a short memory leapfrogged the long one it ranked below: %v", ids(got))
	}
	if kept := FitBudget(hits[1:2], 5); len(kept) != 1 {
		t.Error("the top hit was dropped for being over budget")
	}
	if len(FitBudget(hits, 0)) != 3 {
		t.Error("a zero budget must mean no cap")
	}
}

func ids(hits []Hit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.ID)
	}
	return out
}

func TestSearchHonoursTokenBudget(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	for i := 1; i <= 5; i++ {
		confirmMemory(t, store, strings.Repeat("shared deployment note ", 10)+string(rune('a'+i)), day(i))
	}
	all, _ := store.Search(ctx, Query{WorkspaceID: "ws", Text: "deployment note"})
	capped, err := store.Search(ctx, Query{WorkspaceID: "ws", Text: "deployment note", TokenBudget: 100})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(capped) == 0 || len(capped) >= len(all) {
		t.Errorf("budget did not shorten the result: %d of %d", len(capped), len(all))
	}
}

func TestParaphrasedProposalMergesInsteadOfStoringTwice(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	original, _, err := store.Propose(ctx, candidate(func(r *Record) {
		r.Content = "the integration test suite requires a running local redis instance on the default localhost port before the continuous integration build for any pull request can pass"
	}), day(1))
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	merged, decision, err := store.Propose(ctx, candidate(func(r *Record) {
		r.Content = "the integration test suite requires a running local redis instance on the default localhost port before the continuous integration build for any pull request will pass"
		r.Evidence = []string{"a second run showed the same connection refused error without redis"}
	}), day(2))
	if err != nil {
		t.Fatalf("propose paraphrase: %v", err)
	}
	if decision.Verdict != VerdictMerge || merged.ID != original.ID {
		t.Fatalf("decision = %+v, merged into %q", decision, merged.ID)
	}
	if len(merged.Evidence) != 2 {
		t.Errorf("the paraphrase's evidence was lost: %v", merged.Evidence)
	}
	all, _ := store.List(ctx, "ws1")
	if len(all) != 1 {
		t.Errorf("%d rows stored for one claim", len(all))
	}
}

func TestAChangedValueIsNotMergedAway(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	for _, port := range []string{"3000", "4000"} {
		stored, decision, err := store.Propose(ctx, candidate(func(r *Record) {
			r.Content = "the local development server for the web interface listens on port " + port + " by default"
		}), day(1))
		if err != nil || decision.Verdict != VerdictStore || stored.ID == "" {
			t.Fatalf("port %s: %+v %v", port, decision, err)
		}
	}
}

func TestPruneRemovesOnlyLongExpiredMemories(t *testing.T) {
	ctx := t.Context()
	store, path := openFileStore(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	propose := func(content string, expires *time.Time) Record {
		t.Helper()
		rec, _, err := store.Propose(ctx, candidate(func(r *Record) {
			r.Content, r.ExpiresAt = content, expires
			r.Evidence = []string{"observed while running the build for " + content}
		}), now.Add(-30*24*time.Hour))
		if err != nil {
			t.Fatalf("propose: %v", err)
		}
		return rec
	}
	old := now.Add(-20 * 24 * time.Hour)
	recent := now.Add(-1 * 24 * time.Hour)
	future := now.Add(24 * time.Hour)
	expiredLong := propose("alpha build exits nonzero today", &old)
	expiredRecent := propose("beta build exits nonzero today", &recent)
	live := propose("gamma build exits nonzero today", &future)
	permanent := propose("delta build exits nonzero today", nil)
	if err := store.Link(ctx, Link{SrcID: expiredLong.ID, DstID: live.ID, Relation: RelationRelatesTo}, now); err != nil {
		t.Fatalf("link: %v", err)
	}

	dry, err := store.Prune(ctx, PruneOptions{Now: now, ExpiredFor: 7 * 24 * time.Hour, DryRun: true})
	if err != nil || dry.Memories != 1 {
		t.Fatalf("dry run: %+v %v", dry, err)
	}
	if _, err := store.Get(ctx, expiredLong.ID); err != nil {
		t.Fatal("a dry run deleted a memory")
	}

	result, err := store.Prune(ctx, PruneOptions{Now: now, ExpiredFor: 7 * 24 * time.Hour})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if result.Memories != 1 || result.Links != 1 {
		t.Errorf("result = %+v", result)
	}
	if _, err := store.Get(ctx, expiredLong.ID); err == nil {
		t.Error("the long-expired memory survived")
	}
	for name, id := range map[string]string{"recently expired": expiredRecent.ID, "live": live.ID, "permanent": permanent.ID} {
		if _, err := store.Get(ctx, id); err != nil {
			t.Errorf("%s memory was pruned: %v", name, err)
		}
	}

	history, _ := store.History(ctx, expiredLong.ID)
	if len(history) == 0 || history[len(history)-1].Action != "prune" {
		t.Errorf("the ledger does not explain the missing row: %+v", history)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	var indexed int
	_ = raw.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories_fts WHERE memory_id = ?`, expiredLong.ID).Scan(&indexed)
	if indexed != 0 {
		t.Error("the pruned memory is still in the search index")
	}
}

func TestPruneSessionsIsOptIn(t *testing.T) {
	ctx := t.Context()
	store, path := openFileStore(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	for i, at := range []time.Time{now.Add(-200 * 24 * time.Hour), now.Add(-2 * 24 * time.Hour)} {
		if _, err := raw.ExecContext(ctx, `INSERT INTO session_events (scope, sequence, type, at, payload, created_at)
            VALUES ('ambient', ?, 'message', ?, '{"body":"webhook retry"}', ?)`, i+1, at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	kept, err := store.Prune(ctx, PruneOptions{Now: now})
	if err != nil || kept.Sessions != 0 {
		t.Fatalf("no retention given, yet: %+v %v", kept, err)
	}
	result, err := store.Prune(ctx, PruneOptions{Now: now, SessionsOlderThan: 90 * 24 * time.Hour})
	if err != nil || result.Sessions != 1 {
		t.Fatalf("prune: %+v %v", result, err)
	}
	hits, _ := store.SearchSessions(ctx, "webhook", 10)
	if len(hits) != 1 {
		t.Errorf("after pruning %d sessions are searchable, want 1 (index must follow the table)", len(hits))
	}
}
