package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/checkpoint"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// MemoryDisclaimer rides along with every retrieved memory.
//
// The text lives in the memory package because the hooks in internal/harness
// retrieve too, and one wording that drifts between the two surfaces is worse
// than none.
const MemoryDisclaimer = memory.Disclaimer

// Deps is what the tools need to do their work.
type Deps struct {
	WorkspaceRoot string
	ToolkitRoot   string
	WorkspaceID   string
	Memory        *memory.Store
	Now           func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now == nil {
		return time.Now().UTC()
	}
	return d.Now().UTC()
}

// NewServer builds the stdio server with the seven-tool surface.
func NewServer(version string, deps Deps) *Server {
	return &Server{
		Name:    "vibe-agent",
		Version: version,
		Tools:   Tools(deps),
	}
}

// Tools is the whole surface. Verifiers are not exposed individually: they run
// inside vibe_verify, so evidence is always recorded with the transition it
// justifies and never handed back for a caller to reinterpret.
func Tools(deps Deps) []Tool {
	return []Tool{
		{
			Name:        "vibe_bootstrap",
			Description: "Load the workspace rules, the active run, and relevant memories. Call before starting work.",
			InputSchema: schema(`{"type":"object","properties":{"slug":{"type":"string","description":"Goal slug, when one is already chosen"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return bootstrap(deps, raw) },
		},
		{
			Name:        "vibe_memory_search",
			Description: "Search confirmed project memory. Results are supporting context, not authority.",
			InputSchema: schema(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"kinds":{"type":"array","items":{"type":"string","enum":["semantic","episodic","correction","preference"]}},"limit":{"type":"integer","minimum":1,"maximum":25}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return searchMemory(deps, raw) },
		},
		{
			Name:        "vibe_memory_propose",
			Description: "Propose an evidence-backed memory. It is stored as proposed; only a verifier result or a human confirms it.",
			InputSchema: schema(`{"type":"object","required":["kind","content","evidence","sourceType"],"properties":{"kind":{"type":"string","enum":["semantic","episodic","correction","preference"]},"content":{"type":"string"},"evidence":{"type":"array","minItems":1,"items":{"type":"string"}},"sourceType":{"type":"string","enum":["command_result","file_content","ci_api","human_statement","review_comment"]},"sourceRef":{"type":"string"},"confidence":{"type":"number","minimum":0,"maximum":1},"tags":{"type":"array","items":{"type":"string"}}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return proposeMemory(deps, raw) },
		},
		{
			Name:        "vibe_run_start",
			Description: "Create a run from a workflow graph.",
			InputSchema: schema(`{"type":"object","required":["slug","goal"],"properties":{"slug":{"type":"string"},"goal":{"type":"string"},"graph":{"type":"string","default":"goal-delivery"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return runStart(deps, raw) },
		},
		{
			Name:        "vibe_run_status",
			Description: "Current node, required action, blockers, and what completing this node requires.",
			InputSchema: schema(`{"type":"object","required":["slug"],"properties":{"slug":{"type":"string"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return runStatus(deps, raw) },
		},
		{
			Name:        "vibe_checkpoint",
			Description: "Record evidence for the current node and advance the graph. Evidence must come from a command exit code, a file assertion, a CI response, or a human approval. A verifier node's check cannot be recorded here; call vibe_verify.",
			InputSchema: schema(`{"type":"object","required":["slug"],"properties":{"slug":{"type":"string"},"check":{"type":"object","required":["name","passed","source"],"properties":{"name":{"type":"string"},"passed":{"type":"boolean"},"skipped":{"type":"boolean"},"source":{"type":"string","enum":["exit_code","file_assert","ci_api","human_event"]},"ref":{"type":"string"}}},"result":{"type":"object","additionalProperties":{"type":"boolean"}},"blocker":{"type":"string"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return runCheckpoint(deps, raw) },
		},
		{
			// No verdict parameter, by design. The tool runs what the workspace's
			// vibe-checks.yaml declares for the current node's check and records
			// what happened. A caller cannot supply the outcome or the command.
			Name:        "vibe_verify",
			Description: "Run the current verifier node's check as declared in vibe-checks.yaml and record what it found. This is the only way a verifier node advances. Takes no verdict: the result is the verifier's.",
			InputSchema: schema(`{"type":"object","required":["slug"],"properties":{"slug":{"type":"string"},"check":{"type":"string","description":"Fail unless the current node writes this check"},"dryRun":{"type":"boolean","description":"Report what would run without running it"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return runVerify(deps, raw) },
		},
	}
}

func schema(raw string) json.RawMessage { return json.RawMessage(raw) }

func bootstrap(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug string `json:"slug"`
	}
	_ = json.Unmarshal(raw, &args)

	out := map[string]any{
		"workspaceRoot": deps.WorkspaceRoot,
		"sourceOfTruth": []string{"repository code and config", "git-backed project rules", "current run state", "retrieved memory", "model assumptions"},
		"rulesFiles":    presentFiles(deps.WorkspaceRoot, "AGENTS.md", "CLAUDE.md", "CLAUDE.local.md", "CURSOR.md"),
		"routerEntry":   relativeIfPresent(deps.WorkspaceRoot, filepath.Join(deps.ToolkitRoot, ".ai-agents", "ROUTER.md")),
		"memoryPolicy":  MemoryDisclaimer,
	}

	if args.Slug != "" {
		if run, err := state.Load(state.ManifestPath(deps.WorkspaceRoot, args.Slug)); err == nil {
			out["run"] = summarize(run)
		}
	}
	if deps.Memory != nil {
		hits, err := deps.Memory.Search(context.Background(), memory.Query{
			WorkspaceID: deps.WorkspaceID, Limit: memory.DefaultLimit,
		})
		if err != nil {
			return nil, err
		}
		out["memories"] = renderHits(hits)
	}
	return out, nil
}

func searchMemory(deps Deps, raw json.RawMessage) (any, error) {
	if deps.Memory == nil {
		return nil, fmt.Errorf("memory store is not available")
	}
	var args struct {
		Query string   `json:"query"`
		Kinds []string `json:"kinds"`
		Limit int      `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	query := memory.Query{WorkspaceID: deps.WorkspaceID, Text: args.Query, Limit: args.Limit}
	for _, kind := range args.Kinds {
		query.Kinds = append(query.Kinds, memory.Kind(kind))
	}
	hits, err := deps.Memory.Search(context.Background(), query)
	if err != nil {
		return nil, err
	}
	return map[string]any{"memories": renderHits(hits), "policy": MemoryDisclaimer}, nil
}

func proposeMemory(deps Deps, raw json.RawMessage) (any, error) {
	if deps.Memory == nil {
		return nil, fmt.Errorf("memory store is not available")
	}
	var args struct {
		Kind       string   `json:"kind"`
		Content    string   `json:"content"`
		Evidence   []string `json:"evidence"`
		SourceType string   `json:"sourceType"`
		SourceRef  string   `json:"sourceRef"`
		Confidence float64  `json:"confidence"`
		Tags       []string `json:"tags"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Confidence == 0 {
		args.Confidence = 0.7
	}

	stored, decision, err := deps.Memory.Propose(context.Background(), memory.Record{
		WorkspaceID: deps.WorkspaceID,
		Kind:        memory.Kind(args.Kind),
		Content:     args.Content,
		Tags:        args.Tags,
		Confidence:  args.Confidence,
		Status:      memory.StatusProposed,
		SourceType:  memory.SourceType(args.SourceType),
		SourceRef:   args.SourceRef,
		Evidence:    args.Evidence,
	}, deps.now())
	if err != nil {
		return nil, err
	}

	out := map[string]any{"verdict": string(decision.Verdict), "reason": decision.Reason}
	if decision.Verdict != memory.VerdictReject {
		out["id"] = stored.ID
		out["status"] = string(stored.Status)
		out["note"] = "Stored as proposed. Confirmation requires a verifier result or a human event."
	}
	return out, nil
}

func runStart(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug  string `json:"slug"`
		Goal  string `json:"goal"`
		Graph string `json:"graph"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Graph == "" {
		args.Graph = "goal-delivery"
	}

	loaded, err := graph.LoadByID(graph.DefaultDir(deps.ToolkitRoot), args.Graph)
	if err != nil {
		return nil, err
	}

	manifest := state.ManifestPath(deps.WorkspaceRoot, args.Slug)
	if _, err := os.Stat(manifest); err == nil {
		return nil, fmt.Errorf("a run already exists at %s", manifest)
	}

	run, err := state.NewRun(args.Slug, args.Goal, loaded.Metadata.ID, loaded.Spec.MaxTransitions, deps.now())
	if err != nil {
		return nil, err
	}
	runner := &loop.Runner{Graph: loaded, Now: deps.now}
	if err := runner.Enter(run); err != nil {
		return nil, err
	}
	if _, err := state.AppendEvent(state.EventLogPath(deps.WorkspaceRoot, args.Slug),
		state.Event{Type: "run_started", Node: run.CurrentNode, At: deps.now()}); err != nil {
		return nil, err
	}
	if err := state.Save(manifest, run); err != nil {
		return nil, err
	}
	return describe(loaded, run), nil
}

func runStatus(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	run, err := state.Load(state.ManifestPath(deps.WorkspaceRoot, args.Slug))
	if err != nil {
		return nil, err
	}
	loaded, err := graph.LoadByID(graph.DefaultDir(deps.ToolkitRoot), run.GraphID)
	if err != nil {
		return nil, err
	}
	return describe(loaded, run), nil
}

// runVerify produces a verifier node's evidence. The handler is short because
// checkpoint.Verify owns the sequence; duplicating it here is how the CLI and
// this tool would drift.
func runVerify(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug   string `json:"slug"`
		Check  string `json:"check"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	request := checkpoint.VerifyRequest{
		WorkspaceRoot: deps.WorkspaceRoot,
		GraphDir:      graph.DefaultDir(deps.ToolkitRoot),
		Slug:          args.Slug,
		Check:         args.Check,
		Now:           deps.now(),
	}

	if args.DryRun {
		resolved, err := checkpoint.Resolve(request)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"dryRun": true, "node": resolved.Node, "check": resolved.Check,
			"verifier": resolved.Kind, "plan": resolved.PlanPath,
			"command": resolved.Entry.Command, "args": resolved.Entry.Args,
			"timeoutSeconds": int(resolved.Timeout.Seconds()),
		}, nil
	}

	result, err := checkpoint.Verify(context.Background(), request)
	if err != nil {
		return nil, err
	}

	applied := result.Applied
	out := describe(applied.Graph, applied.Run)
	out["check"] = result.Check
	out["verifier"] = result.Kind
	out["passed"] = result.Verifier.Check.Passed
	out["evidence"] = result.Verifier.Summary
	if result.Verifier.LogPath != "" {
		out["log"] = result.Verifier.LogPath
	}
	if applied.Duplicate {
		out["duplicate"] = true
		out["note"] = "This exact evidence was the last checkpoint recorded, so nothing advanced."
		return out, nil
	}
	out["transition"] = map[string]any{
		"from": applied.Transition.From, "to": applied.Transition.To,
		"via": applied.Transition.Via, "terminal": applied.Transition.Terminal,
	}
	return out, nil
}

func runCheckpoint(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug  string `json:"slug"`
		Check *struct {
			Name    string `json:"name"`
			Passed  bool   `json:"passed"`
			Skipped bool   `json:"skipped"`
			Source  string `json:"source"`
			Ref     string `json:"ref"`
		} `json:"check"`
		Result  map[string]bool `json:"result"`
		Blocker string          `json:"blocker"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	outcome := loop.Outcome{Result: args.Result, Blocker: args.Blocker}
	if args.Check != nil {
		outcome.Check = &loop.NamedCheck{
			Name: args.Check.Name,
			Check: state.Check{
				Passed:  args.Check.Passed,
				Skipped: args.Check.Skipped,
				Source:  state.CheckSource(args.Check.Source),
				Ref:     args.Check.Ref,
				At:      deps.now(),
			},
		}
	}

	result, err := checkpoint.Apply(checkpoint.Request{
		WorkspaceRoot: deps.WorkspaceRoot,
		GraphDir:      graph.DefaultDir(deps.ToolkitRoot),
		Slug:          args.Slug,
		Outcome:       outcome,
		Now:           deps.now(),
	})
	if err != nil {
		return nil, err
	}

	out := describe(result.Graph, result.Run)
	if result.Duplicate {
		// A tool call that timed out after the write gets retried. Telling the
		// caller its evidence was already recorded is more useful than either
		// advancing twice or reporting a transition that did not happen.
		out["duplicate"] = true
		out["note"] = "This exact evidence was the last checkpoint recorded, so nothing advanced."
		return out, nil
	}
	out["transition"] = map[string]any{
		"from": result.Transition.From, "to": result.Transition.To,
		"via": result.Transition.Via, "terminal": result.Transition.Terminal,
	}
	return out, nil
}

// describe turns run state into the next action, so the host is told what to do
// rather than left to infer it from the graph.
func describe(loaded *graph.Graph, run *state.Run) map[string]any {
	out := summarize(run)
	node, ok := loaded.Node(run.CurrentNode)
	if !ok {
		return out
	}

	action := map[string]any{"kind": string(node.Type), "description": node.Description}
	switch node.Type {
	case graph.NodeAgent:
		action["command"] = node.Command
		action["completion"] = "Do the work, then call vibe_checkpoint. Agent nodes produce no automatic pass."
	case graph.NodeArtifact:
		action["command"] = node.Command
		action["outputs"] = node.Outputs
		action["completion"] = "Every listed output must exist and be non-empty."
	case graph.NodeVerifier:
		action["verifier"] = string(node.Verifier)
		action["check"] = node.Check
		action["completion"] = "Run the verification and report the real result. Evidence must be an exit code, a file assertion, or a CI response."
	case graph.NodeHumanGate:
		action["check"] = node.Check
		action["prompt"] = node.Prompt
		if node.Guards != "" {
			action["guards"] = node.Guards
		}
		action["completion"] = "Ask the human. Only a recorded human approval advances this node."
	case graph.NodeTerminal:
		action["completion"] = "The run is finished."
	}
	out["requiredAction"] = action
	out["graph"] = loaded.Metadata.ID
	return out
}

func summarize(run *state.Run) map[string]any {
	checks := map[string]any{}
	for name, check := range run.Checks {
		checks[name] = map[string]any{
			"passed": check.Passed, "skipped": check.Skipped,
			"source": string(check.Source), "ref": check.Ref,
		}
	}
	out := map[string]any{
		"runId":       run.RunID,
		"slug":        run.Slug,
		"goal":        run.Goal,
		"currentNode": run.CurrentNode,
		"status":      string(run.Status),
		"iteration":   run.Iteration,
		"budget":      run.MaxTransitions,
		"checks":      checks,
	}
	if len(run.Blockers) > 0 {
		blockers := make([]map[string]any, 0, len(run.Blockers))
		for _, blocker := range run.Blockers {
			blockers = append(blockers, map[string]any{
				"node": blocker.Node, "reason": blocker.Reason, "attempts": blocker.Attempts,
			})
		}
		out["blockers"] = blockers
	}
	return out
}

func renderHits(hits []memory.Hit) []map[string]any {
	rendered := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		rendered = append(rendered, map[string]any{
			"id": hit.ID, "kind": string(hit.Kind),
			"content": hit.Content, "confidence": hit.Confidence,
			"evidence": hit.Evidence, "source": hit.SourceRef,
		})
	}
	return rendered
}

func presentFiles(root string, names ...string) []string {
	var present []string
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			present = append(present, name)
		}
	}
	return present
}

func relativeIfPresent(root, path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	if relative, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(path)
}
