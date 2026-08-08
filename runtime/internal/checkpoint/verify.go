package checkpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
	"github.com/ducnd58233/vibe-agent/runtime/internal/verifier"
)

// VerifyRequest asks the runtime to produce a verifier node's evidence.
//
// There is no field for a verdict. That absence is the design: the caller says
// which run to verify, and the runtime decides what happened. A CLI flag or tool
// parameter that could override the result would put completion back in the
// hands of whoever wanted to declare it.
type VerifyRequest struct {
	WorkspaceRoot string
	GraphDir      string
	Slug          string
	// Check, when set, must match the current node's check. It exists so a
	// caller can state its intent and be told when the run has moved on, rather
	// than silently verifying something else.
	Check string
	// Registry defaults to verifier.Default(). Injectable for tests.
	Registry verifier.Registry
	Now      time.Time
}

// VerifyResult is what the verifier produced and what the run did about it.
type VerifyResult struct {
	Check string
	// Kind is the verifier that ran.
	Kind     string
	Verifier verifier.Result
	Applied  *Result
}

// Plan is the resolved decision about how a check will be produced, before
// anything runs. Exposed so a caller can show its work without side effects.
type Plan struct {
	Node     string
	Check    string
	Kind     string
	PlanPath string
	Entry    checkplan.Entry
	Timeout  time.Duration
	Request  verifier.Request
}

// Resolve works out how the current node's check would be produced, without
// producing it. Verify calls this first, so a dry run and a real run cannot
// disagree about what would happen.
func Resolve(req VerifyRequest) (*Plan, error) {
	run, err := state.Load(state.ManifestPath(req.WorkspaceRoot, req.Slug))
	if err != nil {
		return nil, err
	}
	loaded, err := graph.LoadByID(req.GraphDir, run.GraphID)
	if err != nil {
		return nil, err
	}
	node, ok := loaded.Node(run.CurrentNode)
	if !ok {
		return nil, fmt.Errorf("run is at node %q, which graph %q does not define", run.CurrentNode, run.GraphID)
	}
	if node.Type != graph.NodeVerifier {
		return nil, fmt.Errorf("node %q is a %s node, not a verifier; there is nothing to verify here",
			run.CurrentNode, node.Type)
	}
	if req.Check != "" && req.Check != node.Check {
		return nil, fmt.Errorf("node %q writes check %q, not %q", run.CurrentNode, node.Check, req.Check)
	}

	plan, err := checkplan.Load(checkplan.DefaultPath(req.WorkspaceRoot))
	if err != nil {
		return nil, err
	}
	entry, err := plan.Entry(node.Check)
	if err != nil {
		return nil, err
	}
	if entry.Human() {
		return nil, fmt.Errorf("check %q is declared %s in %s, so a person records it: "+
			"vibe-agent checkpoint --slug %s --check %s --source %s --passed",
			node.Check, checkplan.HumanVerifier, plan.Path(), req.Slug, node.Check, state.SourceHumanEvent)
	}

	kind := entry.Verifier
	if kind == "" {
		kind = string(node.Verifier)
	}
	if kind == "" {
		return nil, fmt.Errorf("node %q names no verifier and %s does not either", run.CurrentNode, plan.Path())
	}

	timeout := entry.Timeout()
	if entry.TimeoutSeconds <= 0 && node.TimeoutSeconds > 0 {
		// The graph's bound is the fallback, not the override: the plan belongs to
		// the workspace, which is the thing that knows how long its suite takes.
		timeout = time.Duration(node.TimeoutSeconds) * time.Second
	}

	return &Plan{
		Node:     run.CurrentNode,
		Check:    node.Check,
		Kind:     kind,
		PlanPath: plan.Path(),
		Entry:    entry,
		Timeout:  timeout,
		Request: verifier.Request{
			Check:         node.Check,
			WorkspaceRoot: req.WorkspaceRoot,
			Slug:          req.Slug,
			Timeout:       timeout,
			Command:       entry.Command,
			Args:          entry.Args,
			LogDir:        node.Check,
			Paths:         entry.Paths,
		},
	}, nil
}

// Verify runs the current node's verifier and checkpoints what it found.
//
// This is the only function in the program that produces runtime-origin
// evidence, and it can only do so after a verifier has returned. Everything
// else that writes a check is a caller making a claim.
func Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	resolved, err := Resolve(req)
	if err != nil {
		return nil, err
	}

	registry := req.Registry
	if registry == nil {
		registry = verifier.Default()
	}
	impl, err := registry.Get(resolved.Kind)
	if err != nil {
		return nil, err
	}

	produced, err := impl.Verify(ctx, resolved.Request)
	if err != nil {
		return nil, err
	}

	applied, err := Apply(Request{
		WorkspaceRoot: req.WorkspaceRoot,
		GraphDir:      req.GraphDir,
		Slug:          req.Slug,
		origin:        originRuntime,
		Now:           req.Now,
		Outcome: loop.Outcome{Check: &loop.NamedCheck{
			Name:  resolved.Check,
			Check: produced.Check,
		}},
	})
	if err != nil {
		return nil, err
	}

	return &VerifyResult{
		Check:    resolved.Check,
		Kind:     resolved.Kind,
		Verifier: produced,
		Applied:  applied,
	}, nil
}
