package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/agentstate"
)

// Every prompt used to carry the same run status line, the same "follow the
// current node" sentence, and the same top memories, again, for as long as the
// session lasted. Hook output lands in the transcript, so each repeat is paid
// for on every later turn and adds nothing the model did not already have.
//
// The ledger remembers what a session was told. A line already delivered is not
// sent again until injectRefresh prompts have passed, and a memory already
// delivered is excluded from retrieval so its slot goes to something new.
// Agent-memory systems converge on the same shape: dedupe injected memories by
// id and keep the context append-only, which also keeps the provider's prompt
// cache valid.
//
// It is bounded rather than permanent because hosts compact. A line the host
// dropped in a compaction would otherwise never return; SessionStart fires again
// after one, resets the ledger, and the refresh window covers a host that
// truncates silently.

// injectRefresh is how many prompts a delivered line is trusted to still be in
// context. Long enough that a short session never repeats itself, short enough
// that a long one re-states the run it is in before the host has likely
// compacted it away.
const injectRefresh = 12

const injectNamespace = "inject"

type injectLedger struct {
	key    string
	loaded bool
	Prompt int            `json:"prompt"`
	Seen   map[string]int `json:"seen"`
}

// ledgerKey scopes a ledger to one conversation. Two sessions sharing a
// workspace must not suppress each other's context.
func ledgerKey(req Request, body payload) string {
	session := body.sessionKey()
	if session == "" {
		session = "default"
	}
	return string(req.Client) + ":" + session
}

// loadInjectLedger reads this conversation's ledger. Any failure yields an empty
// one, which sends everything: the behaviour from before the ledger existed.
func loadInjectLedger(req Request, body payload) *injectLedger {
	ledger := &injectLedger{key: ledgerKey(req, body), Seen: map[string]int{}}
	raw, ok, err := agentstate.Get(context.Background(), req.WorkspaceRoot, injectNamespace, ledger.key)
	if err != nil || !ok {
		return ledger
	}
	if json.Unmarshal([]byte(raw), ledger) != nil || ledger.Seen == nil {
		ledger.Seen = map[string]int{}
	}
	ledger.loaded = true
	return ledger
}

// resetInjectLedger starts a conversation's ledger over: a new or resumed or
// compacted session has lost whatever the old one delivered.
//
// The old ledger is deleted rather than overwritten: Delete never creates the
// database, so a session starting in a workspace with nothing to remember still
// leaves nothing behind.
func resetInjectLedger(req Request, body payload) *injectLedger {
	ledger := &injectLedger{key: ledgerKey(req, body), Seen: map[string]int{}}
	_ = agentstate.Delete(context.Background(), req.WorkspaceRoot, injectNamespace, ledger.key)
	return ledger
}

// save is best effort. A ledger that cannot be written degrades to repeating
// context, which is wasteful but never wrong.
//
// Nothing is written until there is something to remember, so a hook firing in
// a workspace with no runs and no memories leaves no database behind: reads
// never create state.
func (l *injectLedger) save(req Request) {
	if len(l.Seen) == 0 && !l.loaded {
		return
	}
	if len(l.Seen) > 256 {
		l.prune()
	}
	encoded, err := json.Marshal(l)
	if err != nil {
		return
	}
	_ = agentstate.Set(context.Background(), req.WorkspaceRoot, injectNamespace, l.key, string(encoded))
}

// due reports whether a line should be sent now, and records it when it is.
func (l *injectLedger) due(line string) bool {
	return l.take("line:" + digest(line))
}

func (l *injectLedger) take(key string) bool {
	if last, ok := l.Seen[key]; ok && l.Prompt-last < injectRefresh {
		return false
	}
	l.Seen[key] = l.Prompt
	return true
}

// recentMemories lists the memory ids still presumed to be in context.
func (l *injectLedger) recentMemories() []string {
	var ids []string
	for key, last := range l.Seen {
		if id, ok := strings.CutPrefix(key, "mem:"); ok && l.Prompt-last < injectRefresh {
			ids = append(ids, id)
		}
	}
	return ids
}

func (l *injectLedger) markMemories(ids []string) {
	for _, id := range ids {
		l.Seen["mem:"+id] = l.Prompt
	}
}

// prune drops entries past the refresh window, which could no longer suppress
// anything.
func (l *injectLedger) prune() {
	for key, last := range l.Seen {
		if l.Prompt-last >= injectRefresh {
			delete(l.Seen, key)
		}
	}
}

func digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:6])
}
