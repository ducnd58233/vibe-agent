package checkpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkplan"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
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

func (r VerifyRequest) now() time.Time {
	if r.Now.IsZero() {
		return time.Now().UTC()
	}
	return r.Now.UTC()
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

	// SkipReason, when set, is why nothing will state. A skipped check is recorded
	// as skipped, never as passed, so a reader of the manifest can still tell
	// what was actually exercised.
	SkipReason string
}

// Skipped reports whether resolving decided nothing should state.
func (p *Plan) Skipped() bool { return p.SkipReason != "" }

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

	// The graph's own condition comes first: a node the graph declares out of
	// scope should not need a plan entry at all.
	if reason, skip := loop.New(loaded).SkipReason(run, run.CurrentNode); skip {
		return &Plan{Node: run.CurrentNode, Check: node.Check, SkipReason: reason}, nil
	}

	plan, err := checkplan.Load(checkplan.DefaultPath(req.WorkspaceRoot))
	if err != nil {
		return nil, err
	}

	// A plan that does not declare this check is the workspace saying it has no
	// such check. That is a statement in a tracked file, which is why it is
	// allowed to skip rather than stall; deleting the entry is a reviewable diff.
	//
	// It is not the same as having no plan at all, which errors above: absence of
	// the whole file is a workspace that has not been set up, and treating that as
	// "no checks needed" would make an unconfigured repo the most permissive one.
	if !plan.Has(node.Check) {
		return &Plan{
			Node: run.CurrentNode, Check: node.Check, PlanPath: plan.Path(),
			SkipReason: undeclaredCheckReason(plan, node.Check),
		}, nil
	}

	entry, err := plan.Entry(node.Check)
	if err != nil {
		return nil, err
	}
	if entry.Human() {
		// A run with the auto flag set and an Auto entry declared gets a real,
		// falsifiable alternative instead of a person's word — never the other
		// way around. A run without the flag, or a check with no Auto entry,
		// hits the exact error below unchanged: this is the one branch that
		// keeps /goal provably identical to before docs/auto-ship-reviews.
		if run.Flags["auto"] && entry.Auto != nil {
			entry = *entry.Auto
		} else {
			return nil, fmt.Errorf("check %q is declared %s in %s, so a person records it: "+
				"vibe-agent checkpoint --slug %s --check %s --source %s --passed",
				node.Check, checkplan.HumanVerifier, plan.Path(), req.Slug, node.Check, state.SourceHumanEvent)
		}
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
			Runner:        entry.Runner,
			LogDir:        node.Check,
			Paths:         entry.Paths,
			Screen:        entry.Screen,
		},
	}, nil
}

// Verify runs the current node's verifier and checkpoints what it found.
//
// For declared checks, this is the only function in the program that produces
// runtime-origin evidence, and it can only do so after a verifier has returned.
// Apply separately derives an undeclared skip from the tracked plan for an
// opted-in auto run; everything else that writes a check is a caller claim.
func Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	resolved, err := Resolve(req)
	if err != nil {
		return nil, err
	}

	var produced verifier.Result
	if resolved.Skipped() {
		// verifier.Skipped is where a skip becomes evidence: source file_assert,
		// passed false, skipped true. Until this call site existed the function was
		// unreachable, and the only way a check got marked skipped was a caller
		// typing --skipped, which is the hole this whole change closes.
		produced = verifier.Skipped(resolved.SkipReason, req.now())
	} else {
		registry := req.Registry
		if registry == nil {
			registry = verifier.Default()
		}
		impl, err := registry.Get(resolved.Kind)
		if err != nil {
			return nil, err
		}
		if produced, err = impl.Verify(ctx, resolved.Request); err != nil {
			return nil, err
		}
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
	if applied.Duplicate {
		if advanced, advErr := advanceStuckVerifier(req, resolved, applied.Run, applied.Graph); advErr != nil {
			return nil, advErr
		} else if advanced != nil {
			applied = advanced
		}
	}

	return &VerifyResult{
		Check:    resolved.Check,
		Kind:     resolved.Kind,
		Verifier: produced,
		Applied:  applied,
	}, nil
}
