package testutil

import (
	"context"
	"fmt"
	"sync"

	agent "github.com/ducnd58233/vibe-agent/runtime/internal/agent/domain"
)

// ScriptedRunner answers requests from a fixed list, in order.
//
// It exists so a test can exercise whatever drives an agent node without a host
// binary on PATH and without a network. The alternative is skipping those tests
// wherever no host is installed, which is most CI machines, and a test that
// skips in CI is a test that is not run.
type ScriptedRunner struct {
	// Replies are returned in order. Running past the end is an error rather
	// than a repeat: a loop that asked more times than the script expected has
	// done something the test did not describe.
	Replies []agent.Response
	// Err, when set, is returned instead of the next reply.
	Err error

	mu       sync.Mutex
	Requests []agent.Request
}

func (r *ScriptedRunner) Name() string { return "scripted" }

func (r *ScriptedRunner) Run(_ context.Context, req agent.Request) (agent.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Requests = append(r.Requests, req)
	if r.Err != nil {
		return agent.Response{}, r.Err
	}
	index := len(r.Requests) - 1
	if index >= len(r.Replies) {
		return agent.Response{}, fmt.Errorf("scripted runner has %d replies and was asked %d times",
			len(r.Replies), len(r.Requests))
	}
	return r.Replies[index], nil
}

// Asked reports how many requests the runner received.
func (r *ScriptedRunner) Asked() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Requests)
}
