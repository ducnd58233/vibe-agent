package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// DeliveryLedgerFile is the basename under delivery/ in the run directory.
const DeliveryLedgerFile = "LEDGER.md"

// Delivery reads .agent-state/runs/.../delivery/LEDGER.md after a task-delivery
// run performed its outward actions.
//
// The ledger is the host's account of what it did, so on its own it proves only
// that a file in the right shape exists. What makes it a check is the second
// half: every outward action the approved SPEC listed (rows with an OA id) must
// have a row in the ledger. A ledger that quietly leaves out the message the
// person approved, or reports "none" when the SPEC listed three, fails here.
type Delivery struct{}

func (Delivery) Kind() string { return "delivery" }

// DeliveryLedgerPath is where the host agent must keep the delivery ledger.
func DeliveryLedgerPath(workspaceRoot, slug string) string {
	dir := state.RunDir(workspaceRoot, slug)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "delivery", DeliveryLedgerFile)
}

// specActionID matches the first cell of an outward-actions table row, such as
// "| OA1 | send email | ...". A "none" row has no id and so lists no actions.
var specActionID = regexp.MustCompile(`(?im)^\s*\|\s*(OA\d+)\s*\|`)

// ledgerRowID matches the first cell of any table row in the ledger.
var ledgerRowID = regexp.MustCompile(`(?im)^\s*\|\s*([a-z0-9_-]+)\s*\|`)

func (Delivery) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, errors.New("delivery verifier needs a slug")
	}
	ledgerPath := DeliveryLedgerPath(req.WorkspaceRoot, req.Slug)
	result, err := verifyReviewFile(req, "delivery", ledgerPath,
		"LEDGER.md missing; write the delivery ledger before verify",
		"delivery")
	if err != nil || !result.Check.Passed {
		return result, err
	}

	manifest := state.ManifestPath(req.WorkspaceRoot, req.Slug)
	run, err := state.Load(manifest)
	if err != nil {
		return Result{}, fmt.Errorf("read run state: %w", err)
	}
	if run.Date == "" || run.Version < 1 {
		return failResult(relativeTo(req.WorkspaceRoot, manifest),
			"run has no docs location, so the SPEC's outward actions cannot be read", time.Now().UTC()), nil
	}
	specPath := filepath.Join(sharedworkspace.DocsDirAt(req.WorkspaceRoot, run.Date, run.Slug, run.Version),
		"SPEC-"+run.Date+".md")
	spec, err := os.ReadFile(filepath.Clean(specPath))
	if errors.Is(err, os.ErrNotExist) {
		return failResult(relativeTo(req.WorkspaceRoot, specPath),
			"SPEC missing; the outward actions it lists cannot be checked against the ledger", time.Now().UTC()), nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", relativeTo(req.WorkspaceRoot, specPath), err)
	}

	inLedger := map[string]bool{}
	for _, match := range ledgerRowID.FindAllStringSubmatch(result.Detail, -1) {
		inLedger[strings.ToUpper(match[1])] = true
	}
	var missing []string
	seen := map[string]bool{}
	for _, match := range specActionID.FindAllStringSubmatch(string(spec), -1) {
		id := strings.ToUpper(match[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		if !inLedger[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return failResult(relativeTo(req.WorkspaceRoot, ledgerPath),
			"ledger has no row for SPEC outward action(s): "+strings.Join(missing, ", "), time.Now().UTC()), nil
	}
	return result, nil
}
