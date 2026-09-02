package auto

import (
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/autoconfig"
	"github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

const (
	// MergeGateNode is the gate immediately before a merge.
	MergeGateNode = "approve_merge"
	// MergeCheckName is the evidence key for merge approval.
	MergeCheckName = "merge_approved"
)

// ApproveMerge records the file-backed merge approval for an opted-in auto run.
//
// The caller still owns the transition and event journal. Keeping those writes
// outside this helper lets checkpoint apply the same transition path whether a
// gate was answered from a document or from the workspace opt-in.
func ApproveMerge(workspaceRoot string, current *run.Run, now time.Time) (string, error) {
	if current == nil {
		return "", fmt.Errorf("auto merge approval needs a run")
	}
	if !current.Flags["auto"] {
		return "", fmt.Errorf("run %q is not on the auto path", current.Slug)
	}
	if current.CurrentNode != MergeGateNode {
		return "", fmt.Errorf("run %q is at node %q, not %s",
			current.Slug, current.CurrentNode, MergeGateNode)
	}

	config, found, err := autoconfig.Load(workspaceRoot)
	if err != nil {
		return "", err
	}
	if !found || !config.MayMerge() {
		return "", nil
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}
	ref := ApprovalReference(workspaceRoot, config)
	if err := current.SetCheckAt(MergeCheckName, run.Check{
		Passed: true,
		Source: run.SourceFileAssert,
		Ref:    ref,
		At:     now,
	}, now); err != nil {
		return "", err
	}
	return ref, nil
}

// ApprovalReference identifies the opt-in answer used for merge approval.
func ApprovalReference(workspaceRoot string, config *autoconfig.Config) string {
	return fmt.Sprintf("%s merge=%t sha256=%s",
		autoconfig.Path(workspaceRoot), config.MayMerge(), config.Digest())
}
