package harness

import (
	"context"
	"os"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
)

// RecallLimit is how many memories ride along with a hook.
//
// Smaller than the store's own default on purpose. A hook injects on every
// session and every prompt, so its budget is a handful of lines, not a page.
const RecallLimit = 5

// recall renders the memories worth putting in front of this turn.
//
// Retrieval lives here, in a hook, rather than only behind vibe_memory_search,
// because a tool call is the model's decision and this is not meant to be.
//
// Every failure is silent. Memory is supporting context; a workspace without a
// database, or with one this process cannot read, gets a session with no
// memories rather than a session that will not start.
func recall(workspaceRoot, query string) string {
	store, opened := openMemory(workspaceRoot)
	if !opened {
		return ""
	}
	defer store.Close()

	hits, err := store.Search(context.Background(), memory.Query{
		WorkspaceID: workspaceRoot,
		Text:        query,
		Limit:       RecallLimit,
	})
	if err != nil || len(hits) == 0 {
		return ""
	}

	lines := []string{"Retrieved memory. " + memory.Disclaimer}
	for _, hit := range hits {
		lines = append(lines, "  - "+singleLine(hit.Record.Content))
	}
	return strings.Join(lines, "\n")
}

// openMemory opens the store only when it already exists.
//
// A hook fires in every workspace the host is pointed at, including ones that
// have never used this toolkit. Creating a database in each of them would make
// a read into a side effect. The write path in journal.go creates it; reads
// never do.
func openMemory(workspaceRoot string) (*memory.Store, bool) {
	path := memory.DBPath(workspaceRoot)
	if _, err := os.Stat(path); err != nil {
		return nil, false
	}
	store, err := memory.OpenAt(path)
	if err != nil {
		return nil, false
	}
	return store, true
}

// singleLine flattens a memory so one record stays one bullet.
func singleLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
