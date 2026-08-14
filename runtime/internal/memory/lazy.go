package memory

import (
	"context"
	"os"
	"sync"
)

// Lazy hands out the store while keeping the rule the package states elsewhere:
// reads never create one.
//
// The rule had two enforcers and one gap. recall in the hooks checks the file
// exists first, and doctor was fixed to do the same after it left .agent-state/
// in directories with nothing to do with the toolkit. The MCP server did not:
// it opened eagerly at startup, before any tool was called.
//
// That gap belongs to the two hosts with no hook system. Codex and opencode
// reach the control plane only over MCP, and their host starts the server in
// every workspace it opens, so every repository either of them was pointed at
// got an empty database it never asked for.
type Lazy struct {
	root string

	mu     sync.Mutex
	store  *Store
	closed bool
}

// NewLazy defers the decision until a caller says whether it is reading or
// writing.
func NewLazy(workspaceRoot string) *Lazy {
	return &Lazy{root: workspaceRoot}
}

// Adopt wraps a store that is already open, for callers that opened one
// themselves and for tests that supply a seeded one.
func Adopt(store *Store) *Lazy {
	return &Lazy{store: store}
}

// Read returns the store if this workspace has one, and nil if it does not.
//
// Nil is a normal answer, not a failure: a workspace that has never stored
// anything has nothing to retrieve, and saying so costs nothing. The callers
// already treat a nil store as "no memories".
func (l *Lazy) Read(ctx context.Context) *Store {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.store != nil || l.closed {
		return l.store
	}
	if _, err := os.Stat(DBPath(l.root)); err != nil {
		return nil
	}
	store, err := OpenAt(ctx, DBPath(l.root))
	if err != nil {
		return nil
	}
	l.store = store
	return store
}

// Write returns the store, creating it if this is the first thing stored here.
func (l *Lazy) Write(ctx context.Context) (*Store, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.store != nil {
		return l.store, nil
	}
	store, err := Open(ctx, l.root)
	if err != nil {
		return nil, err
	}
	l.store = store
	return store, nil
}

// Close releases the store if one was ever opened.
func (l *Lazy) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.store == nil {
		return nil
	}
	err := l.store.Close()
	l.store = nil
	return err
}
