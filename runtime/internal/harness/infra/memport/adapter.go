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

// Recall renders memories worth putting in front of this turn, or "" when none.
func Recall(workspaceRoot, query string) string {
	store, opened := openExisting(workspaceRoot)
	if !opened {
		return ""
	}
	defer func() { _ = store.Close() }()

	hits, err := store.Search(context.Background(), memory.Query{
		WorkspaceID: memory.WorkspaceKey(workspaceRoot),
		Text:        query,
		Limit:       RecallLimit,
	})
	if err != nil || len(hits) == 0 {
		return ""
	}

	lines := []string{"Retrieved memory. " + memory.Disclaimer}
	for _, hit := range hits {
		lines = append(lines, "  - "+singleLine(hit.Content))
	}
	return strings.Join(lines, "\n")
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
