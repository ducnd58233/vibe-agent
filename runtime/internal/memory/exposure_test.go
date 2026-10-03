package memory

import (
	"testing"
	"time"
)

func TestExposureIsCreditedOnceAndOnlyBeforeTheEvidence(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	early := confirmMemory(t, store, "the deploy job reads its token from the vault agent", day(1))
	late := confirmMemory(t, store, "the release notes are generated from merged pull requests", day(1))

	// Repeated exposure of one memory in one run is still one open exposure.
	for i := 0; i < 3; i++ {
		if err := store.RecordExposures(ctx, []string{early.ID}, []string{"run-1"}, day(2)); err != nil {
			t.Fatal(err)
		}
	}
	// Shown after the evidence: it cannot have contributed to it.
	if err := store.RecordExposures(ctx, []string{late.ID}, []string{"run-1"}, day(4)); err != nil {
		t.Fatal(err)
	}

	credited, err := store.CreditExposures(ctx, "run-1", "check unit passed", day(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(credited) != 1 || credited[0] != early.ID {
		t.Fatalf("credited = %v, want only the memory shown before the pass", credited)
	}
	if got, _ := store.Get(ctx, early.ID); got.UsedCount != 1 {
		t.Errorf("used_count = %d, want 1 however often it was shown", got.UsedCount)
	}

	again, _ := store.CreditExposures(ctx, "run-1", "check e2e passed", day(3))
	if len(again) != 0 {
		t.Errorf("a consumed exposure was credited again: %v", again)
	}
	other, _ := store.CreditExposures(ctx, "run-2", "check unit passed", day(5))
	if len(other) != 0 {
		t.Errorf("another run's success credited this run's exposure: %v", other)
	}
}

func TestAMemoryClosedAfterBeingShownEarnsNothing(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	rec := confirmMemory(t, store, "integration tests talk to the staging database directly", day(1))
	if err := store.RecordExposures(ctx, []string{rec.ID}, []string{"run-1"}, day(2)); err != nil {
		t.Fatal(err)
	}
	if err := store.Invalidate(ctx, rec.ID, day(2).Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	credited, err := store.CreditExposures(ctx, "run-1", "check unit passed", day(3))
	if err != nil || len(credited) != 0 {
		t.Fatalf("credited %v (%v): a retracted memory was rewarded", credited, err)
	}
}

// Verified reuse is what promotion reads, so the loop from retrieval to a
// reviewable rule is closed without anyone calling RecordUse by hand.
func TestVerifiedReuseFeedsPromotion(t *testing.T) {
	ctx := t.Context()
	store, _ := openFileStore(t)
	rec := confirmMemory(t, store, "the api client must retry on 429 with the server's retry-after", day(1))
	for i := 0; i < PromotionThreshold; i++ {
		run := "run-" + string(rune('a'+i))
		if err := store.RecordExposures(ctx, []string{rec.ID}, []string{run}, day(2)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreditExposures(ctx, run, "check unit passed", day(3)); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := store.List(ctx, "ws")
	if promotions := ProposePromotions(all); len(promotions) != 1 {
		t.Errorf("promotions = %d after %d verified reuses, want 1", len(promotions), PromotionThreshold)
	}
}
