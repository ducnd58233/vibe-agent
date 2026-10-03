package harness

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/harness/infra/memport"
)

// RecallLimit is how many memories ride along with a hook.
// Re-exported from the memory adapter so existing call sites stay stable.
const RecallLimit = memport.RecallLimit

// recall renders the memories worth putting in front of this turn.
//
// Retrieval lives here, in a hook, rather than only behind vibe_memory_search,
// because a tool call is the model's decision and this is not meant to be.
//
// Every failure is silent. Memory is supporting context; a workspace without a
// database, or with one this process cannot read, gets a session with no
// memories rather than a session that will not start.
//
// A non-nil ledger excludes memories this session was already given and records
// the ones returned, so each prompt spends its slots on something new.
func recall(workspaceRoot, query string, ledger *injectLedger) string {
	opts := memport.RecallOptions{TokenBudget: memport.RecallTokenBudget}
	if ledger != nil {
		opts.Exclude = ledger.recentMemories()
	}
	recalled := memport.RecallWith(workspaceRoot, query, opts)
	if ledger != nil {
		ledger.markMemories(recalled.IDs)
	}
	return recalled.Text
}
