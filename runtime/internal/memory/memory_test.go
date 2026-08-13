package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func at() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) }

func openStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenAt(context.Background(), filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func candidate(mutate ...func(*Record)) Record {
	record := Record{
		WorkspaceID: "ws1",
		Kind:        KindEpisodic,
		Content:     "Integration tests require Redis on localhost:6379.",
		Confidence:  0.95,
		Status:      StatusProposed,
		SourceType:  SourceCommandResult,
		Evidence:    []string{"make integration-test failed with connection refused"},
	}
	for _, apply := range mutate {
		apply(&record)
	}
	return record
}

// The six policy cases named in the spec.
func TestPolicyCases(t *testing.T) {
	ctx := context.Background()

	t.Run("secret candidate is rejected", func(t *testing.T) {
		store := openStore(t)
		for _, content := range []string{
			"The deploy uses api_key = sk-abcdef0123456789abcdef",
			"Set AWS_SECRET_ACCESS_KEY before running the suite",
			"Auth header is Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6",
		} {
			_, decision, err := store.Propose(ctx, candidate(func(r *Record) { r.Content = content }), at())
			if err != nil {
				t.Fatalf("Propose: %v", err)
			}
			if decision.Verdict != VerdictReject {
				t.Errorf("stored a credential: %q -> %s", content, decision.Verdict)
			}
		}
	})

	t.Run("temporary task detail is rejected", func(t *testing.T) {
		store := openStore(t)
		for _, content := range []string{
			"Currently working on the webhook retry branch",
			"TODO: split the migration before merging",
			"On branch feature/idempotency the tests are red",
		} {
			_, decision, err := store.Propose(ctx, candidate(func(r *Record) { r.Content = content }), at())
			if err != nil {
				t.Fatalf("Propose: %v", err)
			}
			if decision.Verdict != VerdictReject {
				t.Errorf("stored task state as durable memory: %q", content)
			}
		}
	})

	t.Run("evidence-backed convention is proposed", func(t *testing.T) {
		store := openStore(t)
		stored, decision, err := store.Propose(ctx, candidate(), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if decision.Verdict != VerdictStore {
			t.Fatalf("verdict = %s (%s), want store", decision.Verdict, decision.Reason)
		}
		if stored.Status != StatusProposed {
			t.Errorf("status = %q, want proposed", stored.Status)
		}
	})

	t.Run("duplicate is merged", func(t *testing.T) {
		store := openStore(t)
		first, _, err := store.Propose(ctx, candidate(), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		second, decision, err := store.Propose(ctx, candidate(func(r *Record) {
			r.Evidence = []string{"docker compose up redis, then the suite passed"}
		}), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if decision.Verdict != VerdictMerge {
			t.Fatalf("verdict = %s, want merge", decision.Verdict)
		}
		if second.ID != first.ID {
			t.Errorf("merge created a new record %s instead of folding into %s", second.ID, first.ID)
		}
		if len(second.Evidence) != 2 {
			t.Errorf("evidence not merged: %v", second.Evidence)
		}
	})

	t.Run("superseded convention goes stale", func(t *testing.T) {
		store := openStore(t)
		old, _, err := store.Propose(ctx, candidate(func(r *Record) {
			r.Content = "The suite runs against Postgres 14."
		}), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if _, err := store.Confirm(ctx, old.ID, SourceCommandResult, "run:1/event:2", at()); err != nil {
			t.Fatalf("Confirm: %v", err)
		}

		replacement, _, err := store.Propose(ctx, candidate(func(r *Record) {
			r.Content = "The suite runs against Postgres 16."
			r.SupersedesID = old.ID
		}), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if _, err := store.Confirm(ctx, replacement.ID, SourceCommandResult, "run:2/event:9", at()); err != nil {
			t.Fatalf("Confirm: %v", err)
		}

		retired, err := store.Get(ctx, old.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if retired.Status != StatusStale {
			t.Errorf("superseded record status = %q, want stale", retired.Status)
		}
	})

	t.Run("another workspace is not retrieved", func(t *testing.T) {
		store := openStore(t)
		mine, _, err := store.Propose(ctx, candidate(), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if _, err := store.Confirm(ctx, mine.ID, SourceCommandResult, "", at()); err != nil {
			t.Fatalf("Confirm: %v", err)
		}
		theirs, _, err := store.Propose(ctx, candidate(func(r *Record) {
			r.WorkspaceID = "ws2"
			r.Content = "Their integration tests require Kafka on localhost:9092."
		}), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if _, err := store.Confirm(ctx, theirs.ID, SourceCommandResult, "", at()); err != nil {
			t.Fatalf("Confirm: %v", err)
		}

		hits, err := store.Search(ctx, Query{WorkspaceID: "ws1", Text: "localhost"})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		for _, hit := range hits {
			if hit.Record.WorkspaceID != "ws1" {
				t.Errorf("search returned a memory from %q", hit.Record.WorkspaceID)
			}
		}
		if len(hits) != 1 {
			t.Errorf("got %d hits, want only the ws1 memory", len(hits))
		}
	})
}

// Procedural memory is refused with a message that says where it belongs,
// rather than a generic validation error.
func TestProceduralMemoryIsRefusedWithGuidance(t *testing.T) {
	store := openStore(t)
	_, decision, err := store.Propose(context.Background(),
		candidate(func(r *Record) { r.Kind = KindProcedural }), at())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if decision.Verdict != VerdictReject {
		t.Fatal("procedural memory was stored")
	}
	if !strings.Contains(decision.Reason, "SKILL.md") {
		t.Errorf("rejection does not say where it belongs: %q", decision.Reason)
	}
}

func TestEvidenceFreeCandidateIsRejected(t *testing.T) {
	store := openStore(t)
	for _, evidence := range [][]string{nil, {}, {"ok"}} {
		_, decision, err := store.Propose(context.Background(),
			candidate(func(r *Record) { r.Evidence = evidence }), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if decision.Verdict != VerdictReject {
			t.Errorf("stored a claim with evidence %v", evidence)
		}
	}
}

func TestHedgedClaimIsRejected(t *testing.T) {
	store := openStore(t)
	_, decision, err := store.Propose(context.Background(),
		candidate(func(r *Record) { r.Content = "This project probably needs Redis." }), at())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if decision.Verdict != VerdictReject {
		t.Error("a hedged guess was stored as memory")
	}
}

// The gate is the point. A caller cannot arrive already confirmed.
func TestProposeRefusesAConfirmedCandidate(t *testing.T) {
	store := openStore(t)
	_, decision, err := store.Propose(context.Background(),
		candidate(func(r *Record) { r.Status = StatusConfirmed }), at())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if decision.Verdict != VerdictReject {
		t.Error("a candidate claiming to be confirmed was accepted")
	}
}

func TestConfirmDemandsRealProvenance(t *testing.T) {
	store := openStore(t)
	stored, _, err := store.Propose(context.Background(), candidate(), at())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := store.Confirm(context.Background(), stored.ID, "model_inference", "", at()); err == nil {
		t.Error("Confirm accepted model_inference as provenance")
	}
}

// Retrieval returns knowledge, not candidates.
func TestSearchExcludesProposedMemoriesByDefault(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	if _, _, err := store.Propose(ctx, candidate(), at()); err != nil {
		t.Fatalf("Propose: %v", err)
	}

	hits, err := store.Search(ctx, Query{WorkspaceID: "ws1", Text: "Redis"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("got %d hits; proposed memories are candidates, not knowledge", len(hits))
	}

	withProposed, err := store.Search(ctx, Query{
		WorkspaceID: "ws1", Text: "Redis", Statuses: []Status{StatusProposed},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(withProposed) != 1 {
		t.Errorf("got %d hits when asking for proposed, want 1", len(withProposed))
	}
}

func TestSearchRanksAndCaps(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)

	contents := []string{
		"Integration tests require Redis on localhost:6379.",
		"The Redis container must be healthy before migrations run.",
		"Frontend builds need Node 22 and pnpm.",
	}
	for _, content := range contents {
		stored, _, err := store.Propose(ctx, candidate(func(r *Record) { r.Content = content }), at())
		if err != nil {
			t.Fatalf("Propose: %v", err)
		}
		if _, err := store.Confirm(ctx, stored.ID, SourceCommandResult, "", at()); err != nil {
			t.Fatalf("Confirm: %v", err)
		}
	}

	hits, err := store.Search(ctx, Query{WorkspaceID: "ws1", Text: "Redis"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits for Redis, want the 2 that mention it", len(hits))
	}
	for _, hit := range hits {
		if !strings.Contains(hit.Record.Content, "Redis") {
			t.Errorf("irrelevant hit: %q", hit.Record.Content)
		}
	}

	capped, err := store.Search(ctx, Query{WorkspaceID: "ws1", Limit: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(capped) != 1 {
		t.Errorf("limit ignored: got %d hits", len(capped))
	}
}

// Punctuation in a phrase must not be read as FTS syntax.
func TestSearchHandlesPunctuationInTerms(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	stored, _, err := store.Propose(ctx, candidate(), at())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := store.Confirm(ctx, stored.ID, SourceCommandResult, "", at()); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if _, err := store.Search(ctx, Query{WorkspaceID: "ws1", Text: "localhost:6379"}); err != nil {
		t.Fatalf("a term with a colon broke the query: %v", err)
	}
}

func TestSearchRequiresAWorkspace(t *testing.T) {
	store := openStore(t)
	if _, err := store.Search(context.Background(), Query{Text: "anything"}); err == nil {
		t.Error("Search ran without a workspace filter")
	}
}

// Promotion proposes; it never writes the rule.
func TestPromotionIsProposedAfterRepeatedUse(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	stored, _, err := store.Propose(ctx, candidate(), at())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := store.Confirm(ctx, stored.ID, SourceCommandResult, "", at()); err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	for i := 0; i < PromotionThreshold-1; i++ {
		if err := store.RecordUse(ctx, stored.ID, at()); err != nil {
			t.Fatalf("RecordUse: %v", err)
		}
	}
	records, err := store.List(ctx, "ws1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := ProposePromotions(records); len(got) != 0 {
		t.Errorf("promoted after %d uses, threshold is %d", PromotionThreshold-1, PromotionThreshold)
	}

	if err := store.RecordUse(ctx, stored.ID, at()); err != nil {
		t.Fatalf("RecordUse: %v", err)
	}
	records, err = store.List(ctx, "ws1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	promotions := ProposePromotions(records)
	if len(promotions) != 1 {
		t.Fatalf("got %d promotions after %d uses, want 1", len(promotions), PromotionThreshold)
	}
	if promotions[0].Target == "" {
		t.Error("promotion does not say where the rule should live")
	}
}

func TestExpiredMemoriesAreNotRetrieved(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	expired := time.Now().UTC().Add(-time.Hour)
	stored, _, err := store.Propose(ctx, candidate(func(r *Record) { r.ExpiresAt = &expired }), at())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := store.Confirm(ctx, stored.ID, SourceCommandResult, "", at()); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	hits, err := store.Search(ctx, Query{WorkspaceID: "ws1", Text: "Redis"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("got %d hits; an expired memory was retrieved", len(hits))
	}
}

func TestStateDirIsWorkspaceLocal(t *testing.T) {
	path := DBPath("/repo")
	if filepath.Base(filepath.Dir(path)) != StateDirName {
		t.Errorf("DBPath = %q, want it under %s", path, StateDirName)
	}
}
