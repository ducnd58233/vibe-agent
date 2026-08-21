package verifier

import (
	"os"
	"path/filepath"
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shipdecision"
)

func writeDecision(t *testing.T, root, slug, body string) {
	t.Helper()
	path := shipdecision.Path(root, slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestShipDecisionPassesOnGO(t *testing.T) {
	root := t.TempDir()
	writeDecision(t, root, "demo", "Ship Decision: GO\nSpecialist: code-reviewer -> PASS\n")

	result, err := ShipDecision{}.Verify(t.Context(), Request{WorkspaceRoot: root, Slug: "demo"})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Errorf("did not pass: %s", result.Summary)
	}
	if result.Check.Source != state.SourceFileAssert {
		t.Errorf("source = %q, want file_assert", result.Check.Source)
	}
}

func TestShipDecisionFailsOnNOGO(t *testing.T) {
	root := t.TempDir()
	writeDecision(t, root, "demo", "Ship Decision: NO-GO\nBLOCKER: missing tests\n")

	result, err := ShipDecision{}.Verify(t.Context(), Request{WorkspaceRoot: root, Slug: "demo"})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("a NO-GO decision passed")
	}
}

// Fail closed: /ship simply not having run yet must not pass, and must not be
// reported as an error either — it is an ordinary not-yet-true fact.
func TestShipDecisionFailsClosedOnAMissingFile(t *testing.T) {
	result, err := ShipDecision{}.Verify(t.Context(), Request{WorkspaceRoot: t.TempDir(), Slug: "demo"})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("a missing decision file passed")
	}
}

func TestShipDecisionFailsClosedOnAMalformedFile(t *testing.T) {
	root := t.TempDir()
	writeDecision(t, root, "demo", "not a decision file at all\n")

	result, err := ShipDecision{}.Verify(t.Context(), Request{WorkspaceRoot: root, Slug: "demo"})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("a malformed decision file passed")
	}
}

func TestShipDecisionNeedsASlug(t *testing.T) {
	if _, err := (ShipDecision{}).Verify(t.Context(), Request{WorkspaceRoot: t.TempDir()}); err == nil {
		t.Fatal("Verify accepted an empty slug")
	}
}

func TestTheRegistryOffersShipDecision(t *testing.T) {
	v, err := Default().Get("shipdecision")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.Kind() != "shipdecision" {
		t.Errorf("Kind() = %q, want shipdecision", v.Kind())
	}
}
