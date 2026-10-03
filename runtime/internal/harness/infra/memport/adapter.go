// Package memport adapts the memory store for harness hooks.
//
// Journal failure proposals and session recall used to call memory.Open from
// the harness package root. Keeping those calls here leaves gate/danger/hook
// envelopes free of store details while the memory module stays the owner of
// schema and Propose/Search rules.
package memport

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/tokenest"
)

// RecallLimit is how many memories ride along with a hook.
//
// Smaller than the store's own default on purpose. A hook injects on every
// session and every prompt, so its budget is a handful of lines, not a page.
const RecallLimit = 5

// FailureMemoryLife is how long a recorded command failure stays retrievable.
//
// "go build ./... exits 2" is true about a moment, not about the repository.
// Left permanent it becomes the stale memory the whole design warns about, so
// it retires itself instead of waiting to be contradicted.
const FailureMemoryLife = 7 * 24 * time.Hour

// FailureProposal is the observation a failed shell command leaves for memory.
type FailureProposal struct {
	WorkspaceRoot string
	Client        string
	Slug          string
	Node          string
	Command       string
	ExitCode      *int
	Stderr        string
	SourceRef     string
	Interrupted   bool
}

// ProposeFailure writes a command-failure memory when the store accepts it.
// Failures are silent: a hook must not block a tool call over bookkeeping.
func ProposeFailure(p FailureProposal) {
	if p.Interrupted {
		return
	}
	store, err := memory.Open(context.Background(), p.WorkspaceRoot)
	if err != nil {
		return
	}
	defer func() { _ = store.Close() }()

	evidence := []string{failureContext(p.Slug, p.Node, p.Command, p.ExitCode)}
	if detail := truncate(singleLine(p.Stderr), 300); detail != "" {
		evidence = append(evidence, detail)
	}

	now := time.Now().UTC()
	expires := now.Add(FailureMemoryLife)
	ctx := context.Background()

	stored, decision, err := store.Propose(ctx, memory.Record{
		WorkspaceID: memory.WorkspaceKey(p.WorkspaceRoot),
		Kind:        memory.KindEpisodic,
		Content:     fmt.Sprintf("%s %s in this workspace", p.Command, exitsPhrase(p.ExitCode)),
		Tags:        failureTags(p.Slug),
		Confidence:  0.6,
		SourceType:  memory.SourceCommandResult,
		SourceRef:   p.SourceRef,
		Evidence:    evidence,
		ExpiresAt:   &expires,
		CreatedBy:   memory.Author(p.Client, ""),
	}, now)
	if err != nil || decision.Verdict == memory.VerdictReject {
		return
	}
	_, _ = store.Confirm(ctx, stored.ID, memory.SourceCommandResult, p.SourceRef, now)
}

// RecallTokenBudget caps what one retrieval may add to a prompt. Five memories
// is a count, not a cost; a budget keeps five paragraphs from costing what
// five facts would. Per-memory text is also capped, so one long memory cannot
// take the whole allowance.
const RecallTokenBudget = 350

// recallLineChars caps one memory's line. Content can be 2000 characters; a
// prompt-time reminder needs the claim, and the id lets a model ask for the rest.
const recallLineChars = 280

// RecallOptions tunes one retrieval.
type RecallOptions struct {
	// Exclude skips memories already in front of the model.
	Exclude []string
	// TokenBudget caps the estimated tokens returned. Zero means RecallLimit
	// alone.
	TokenBudget int
	// ExposeTo names the runs in flight. Each returned memory is recorded as
	// exposed to them, which is half of what earns it a use; the other half is
	// the run later passing a verified check.
	ExposeTo []string
}

// Recalled is rendered text and the ids it carries, so a caller can remember
// what it delivered.
type Recalled struct {
	Text string
	IDs  []string
}

// RecallWith renders memories worth putting in front of this turn, or empty text
// when none, skipping excluded ids and fitting a token budget.
func RecallWith(workspaceRoot, query string, opts RecallOptions) Recalled {
	store, opened := openExisting(workspaceRoot)
	if !opened {
		return Recalled{}
	}
	defer func() { _ = store.Close() }()

	hits, err := store.Search(context.Background(), memory.Query{
		WorkspaceID: memory.WorkspaceKey(workspaceRoot),
		Text:        query,
		Limit:       RecallLimit,
		Exclude:     opts.Exclude,
		TokenBudget: opts.TokenBudget,
	})
	if err != nil || len(hits) == 0 {
		return Recalled{}
	}

	lines := []string{"Retrieved memory. " + memory.Disclaimer}
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		lines = append(lines, "  - "+tokenest.TruncateWords(singleLine(hit.Content), recallLineChars))
		ids = append(ids, hit.ID)
	}
	// Bookkeeping, so best effort: a failed write costs a use that may have
	// been earned, never the recall itself.
	_ = store.RecordExposures(context.Background(), ids, opts.ExposeTo, time.Now().UTC())
	return Recalled{Text: strings.Join(lines, "\n"), IDs: ids}
}

func openExisting(workspaceRoot string) (*memory.Store, bool) {
	path := memory.DBPath(workspaceRoot)
	if _, err := os.Stat(path); err != nil {
		return nil, false
	}
	store, err := memory.OpenAt(context.Background(), path)
	if err != nil {
		return nil, false
	}
	return store, true
}

func failureContext(slug, node, command string, exit *int) string {
	if slug == "" {
		return fmt.Sprintf("%s %s outside any run", command, exitedPhrase(exit))
	}
	return fmt.Sprintf("%s %s during run %s at node %s",
		command, exitedPhrase(exit), slug, orNotEntered(node))
}

func failureTags(slug string) []string {
	if slug == "" {
		return []string{"command-failure"}
	}
	return []string{"command-failure", slug}
}

func exitedPhrase(exit *int) string {
	if exit == nil {
		return "failed"
	}
	return fmt.Sprintf("exited %d", *exit)
}

func exitsPhrase(exit *int) string {
	if exit == nil {
		return "fails"
	}
	return fmt.Sprintf("exits %d", *exit)
}

func orNotEntered(node string) string {
	if node == "" {
		return "(not entered)"
	}
	return node
}

func singleLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func truncate(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "..."
}
