package checkpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// creditMemories turns this run's memory exposures into reuse, when and only
// when the evidence just recorded is a check the runtime itself watched pass.
//
// That is the strong form of "the memory was used successfully". A memory
// being retrieved is not reuse; a caller asserting a pass is a claim, not a
// result (see origin); a skipped check proved nothing. What remains is a memory
// that was in front of the model during a run, followed by verified success in
// that run, which is the closest observable thing to "it helped" that does not
// ask a model to grade itself.
//
// Best effort, like every piece of memory bookkeeping: the run has already been
// saved, and a failed credit costs a use count, never a checkpoint. A workspace
// with no memory database has nothing exposed and is not given one.
func creditMemories(ctx context.Context, req Request, run *state.Run, at time.Time) {
	check := req.Outcome.Check
	if req.origin != originRuntime || check == nil || !check.Check.Passed || check.Check.Skipped {
		return
	}
	lazy := memory.NewLazy(req.WorkspaceRoot)
	defer func() { _ = lazy.Close() }()
	store := lazy.Read(ctx)
	if store == nil {
		return
	}
	evidence := fmt.Sprintf("run %s check %s passed (%s%s)", run.Slug, check.Name, check.Check.Source, refSuffix(check.Check.Ref))
	_, _ = store.CreditExposures(ctx, run.RunID, evidence, at)
}

func refSuffix(ref string) string {
	if ref == "" {
		return ""
	}
	return ": " + ref
}
