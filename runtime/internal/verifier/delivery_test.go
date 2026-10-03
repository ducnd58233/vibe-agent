package verifier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

const deliverySpecWithActions = `---
slug: d
date: 2026-08-25
version: 1
---

## Outward actions
| ID | Action | Tool or channel | Recipient or target | Exact content or amount |
|----|--------|-----------------|---------------------|-------------------------|
| OA1 | send email | mail | a@example.com | see drafts/1.md |
| OA2 | share file | drive | team | report.md |
`

const deliverySpecNone = `## Outward actions
| ID | Action | Tool or channel | Recipient or target | Exact content or amount |
|----|--------|-----------------|---------------------|-------------------------|
| none | | | | |
`

func deliveryLedger(rows ...string) string {
	return "# Delivery ledger\nstatus: pass\nattempt: 1\n\n" +
		"| ID | Done | Tool reply | result |\n|----|------|------------|--------|\n" +
		strings.Join(rows, "\n") + "\n"
}

// deliveryRun allocates a run, records its docs location, and writes the SPEC
// and the ledger. An empty body skips that file.
func deliveryRun(t *testing.T, slug, spec, ledger string) string {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 8, 25, 14, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, slug, now); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	run, err := state.NewRun(slug, "deliver it", "task-delivery", 60, now)
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	run.Date, run.Version = "2026-08-25", 1
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if spec != "" {
		dir := sharedworkspace.DocsDirAt(root, run.Date, slug, run.Version)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SPEC-"+run.Date+".md"), []byte(spec), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if ledger != "" {
		dir := filepath.Dir(DeliveryLedgerPath(root, slug))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(DeliveryLedgerPath(root, slug), []byte(ledger), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func verifyDelivery(t *testing.T, root, slug string) Result {
	t.Helper()
	result, err := Delivery{}.Verify(t.Context(), Request{Slug: slug, WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return result
}

func TestDeliveryPassesWhenEveryListedActionHasARow(t *testing.T) {
	root := deliveryRun(t, "d-pass", deliverySpecWithActions,
		deliveryLedger("| OA1 | sent | msg-id 42 | pass |", "| OA2 | shared | link ok | pass |"))
	if result := verifyDelivery(t, root, "d-pass"); !result.Check.Passed {
		t.Fatalf("expected pass: %s", result.Summary)
	}
}

func TestDeliveryFailsWhenALedgerRowIsMissing(t *testing.T) {
	root := deliveryRun(t, "d-missing", deliverySpecWithActions,
		deliveryLedger("| OA1 | sent | msg-id 42 | pass |"))
	result := verifyDelivery(t, root, "d-missing")
	if result.Check.Passed {
		t.Fatal("a ledger that omits OA2 passed")
	}
	if !strings.Contains(result.Summary, "OA2") {
		t.Errorf("summary does not name the missing action: %s", result.Summary)
	}
}

func TestDeliveryFailsWhenTheLedgerSaysNoneButTheSpecListedActions(t *testing.T) {
	root := deliveryRun(t, "d-none", deliverySpecWithActions,
		deliveryLedger("| none | no outward actions | n/a | pass |"))
	if result := verifyDelivery(t, root, "d-none"); result.Check.Passed {
		t.Fatal("a ledger claiming none passed against a SPEC that listed two actions")
	}
}

func TestDeliveryPassesWithNoActionsWhenTheSpecListedNone(t *testing.T) {
	root := deliveryRun(t, "d-nothing", deliverySpecNone,
		deliveryLedger("| none | no outward actions | n/a | pass |"))
	if result := verifyDelivery(t, root, "d-nothing"); !result.Check.Passed {
		t.Fatalf("expected pass: %s", result.Summary)
	}
}

func TestDeliveryFailsOnAFailedRowOrAMissingFileOrSpec(t *testing.T) {
	failed := deliveryRun(t, "d-fail", deliverySpecWithActions,
		strings.Replace(deliveryLedger("| OA1 | refused | tool refused | fail |", "| OA2 | shared | ok | pass |"),
			"status: pass", "status: fail", 1))
	if result := verifyDelivery(t, failed, "d-fail"); result.Check.Passed {
		t.Error("a ledger with a failed row passed")
	}

	noLedger := deliveryRun(t, "d-noledger", deliverySpecWithActions, "")
	if result := verifyDelivery(t, noLedger, "d-noledger"); result.Check.Passed {
		t.Error("a missing ledger passed")
	}

	noSpec := deliveryRun(t, "d-nospec", "", deliveryLedger("| none | x | y | pass |"))
	if result := verifyDelivery(t, noSpec, "d-nospec"); result.Check.Passed {
		t.Error("a ledger passed with no SPEC to check it against")
	}
}
