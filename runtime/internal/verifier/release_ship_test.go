package verifier

import (
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

const passingReleaseReview = `# Release readiness
status: pass
attempt: 1

| Gate id | Evidence source | Pointer | result |
|---------|-----------------|---------|--------|
| R1 | file_assert | ship/DECISION.md | pass |
`

// recordShip writes a run manifest holding a ship check with the given result.
func recordShip(t *testing.T, root, slug string, passed bool) {
	t.Helper()
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	run, err := state.NewRun(slug, "ship it", "goal-delivery", 100, now)
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	if err := run.SetCheckAt("ship", state.Check{Passed: passed, Source: state.SourceFileAssert, Ref: "ship/DECISION.md", At: now}, now); err != nil {
		t.Fatalf("SetCheckAt: %v", err)
	}
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// A host writes release/REVIEW.md itself. On the auto path a /ship NO-GO used
// to flow on to release_review, and a release file claiming its ship gate row
// passed was accepted over the ship check the run had recorded as failed.
func TestReleaseRefusesWhenTheRecordedShipCheckFailed(t *testing.T) {
	root := t.TempDir()
	allocateNamedReview(t, root, "rel-nogo", "release", ReleaseReviewFile, passingReleaseReview)
	recordShip(t, root, "rel-nogo", false)

	result, err := Release{}.Verify(t.Context(), Request{Slug: "rel-nogo", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("a release file passed over a failed ship check")
	}
	if !strings.Contains(result.Summary, "ship") {
		t.Errorf("summary %q does not name the ship check", result.Summary)
	}
}

func TestReleaseRefusesWhenNoShipCheckWasRecorded(t *testing.T) {
	root := t.TempDir()
	allocateNamedReview(t, root, "rel-noship", "release", ReleaseReviewFile, passingReleaseReview)

	result, err := Release{}.Verify(t.Context(), Request{Slug: "rel-noship", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Fatal("a release file passed with no ship check on record")
	}
}

func TestReleasePassesWhenShipPassedAndTheFileAgrees(t *testing.T) {
	root := t.TempDir()
	allocateNamedReview(t, root, "rel-go", "release", ReleaseReviewFile, passingReleaseReview)
	recordShip(t, root, "rel-go", true)

	result, err := Release{}.Verify(t.Context(), Request{Slug: "rel-go", WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Fatalf("ship passed and the release file passes, yet: %s", result.Summary)
	}
}
