package verifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shipdecision"
)

// ShipDecision reads /ship's written verdict and reports GO with zero
// blockers as passed.
//
// This exists only to give the auto path a route past a check that stays
// verifier: human by default (docs/auto-ship-reviews). A missing file is not
// an error: /ship may simply not have run yet in this task's cycle, and the
// honest report is "not passed", not a failure that looks like a broken
// verifier. A file that fails to parse is the same: not passed, never a
// default GO on ambiguous input.
type ShipDecision struct{}

func (ShipDecision) Kind() string { return "shipdecision" }

func (ShipDecision) Verify(_ context.Context, req Request) (Result, error) {
	if req.Slug == "" {
		return Result{}, fmt.Errorf("shipdecision verifier needs a slug to find the decision file")
	}

	path := shipdecision.Path(req.WorkspaceRoot, req.Slug)
	decision, err := shipdecision.Load(req.WorkspaceRoot, req.Slug)

	result := Result{
		Check: state.Check{
			Source: state.SourceFileAssert,
			Ref:    relativeTo(req.WorkspaceRoot, path),
			At:     time.Now().UTC(),
		},
	}

	switch {
	case errors.Is(err, os.ErrNotExist):
		result.Summary = "no decision file yet: /ship has not run, or has not written one"
	case err != nil:
		result.Summary = fmt.Sprintf("decision file did not parse: %s", err)
	case !decision.GO:
		result.Summary = fmt.Sprintf("Ship Decision: NO-GO, %d blocker(s)", len(decision.Blockers))
	default:
		result.Check.Passed = true
		result.Summary = fmt.Sprintf("Ship Decision: GO, %d specialist(s) reported", len(decision.Specialists))
	}
	result.Detail = result.Summary
	return result, nil
}
