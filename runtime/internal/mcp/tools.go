package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/auto"
	"github.com/ducnd58233/vibe-agent/runtime/internal/autoconfig"
	"github.com/ducnd58233/vibe-agent/runtime/internal/checkpoint"
	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graph"
	"github.com/ducnd58233/vibe-agent/runtime/internal/graphroute"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/runstart"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
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
	// Memory is opened on demand, so a host that starts this server in every
	// workspace it opens does not leave a database in each one.
	Memory *memory.Lazy
	Now    func() time.Time
	Log    observability.Logger
	// Session tracks the active slug for tools/list narrowing. Shared with the
	// Server so Serve can emit list_changed after a real transition.
	Session *Session
}

// read returns the store for a retrieval, or nil where this workspace has none.
func (d Deps) read() *memory.Store {
	if d.Memory == nil {
		return nil
	}
	return d.Memory.Read(context.Background())
}

func (d Deps) now() time.Time {
	if d.Now == nil {
		return time.Now().UTC()
	}
	return d.Now().UTC()
}

// NewServer builds the stdio server with the MCP tool surface.
func NewServer(version string, deps Deps) *Server {
	if deps.Session == nil {
		deps.Session = &Session{}
	}
	return &Server{
		Name:    "vibe-agent",
		Version: version,
		Tools:   Tools(deps),
		Log:     deps.Log,
		Deps:    deps,
		Session: deps.Session,
	}
}

// Tools is the whole surface. Verifiers are not exposed individually: they run
// inside vibe_verify, so evidence is always recorded with the transition it
// justifies and never handed back for a caller to reinterpret.
func Tools(deps Deps) []Tool {
	return []Tool{
		{
			Name:        "vibe_bootstrap",
			Description: "Call once at the start of a session to load workspace rules, the active run, and relevant memories. Do not call again mid-session once you already have this context.",
			InputSchema: schema(`{"type":"object","properties":{"slug":{"type":"string","description":"Goal slug, when one is already chosen"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return bootstrap(deps, raw) },
		},
		{
			// The eighth tool, on a ground none of the other seven share.
			//
			// Every other tool here is control-plane bookkeeping and costs a
			// slot in the list to buy nothing back. This one returns tokens
			// rather than spending them: a page arrives clipped to a budget and
			// cached, instead of whole and again next time. It defends the thing
			// a short tool list exists to protect, which is the argument for its
			// place in a list that is deliberately short.
			Name: "vibe_fetch",
			Description: "Call to read a URL, local path, or check:<slug>:<name> log as text clipped to a token budget and cached. " +
				"Do not call again for a source already fetched this session unless you need a refresh or a different clipFrom.",
			InputSchema: schema(`{"type":"object","required":["source"],"properties":{"source":{"type":"string","description":"URL, path, or check:<slug>:<name> for a verifier log"},"budget":{"type":"integer","minimum":1,"description":"Approximate token budget for the returned text"},"refresh":{"type":"boolean","description":"Bypass the cache"},"clipFrom":{"type":"string","enum":["head","tail"],"description":"Keep the start (default) or the end of a long document"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return fetchSource(deps, raw) },
		},
		{
			Name:        "vibe_memory_search",
			Description: "Call to check what earlier work already established before repeating research. Do not call for this run's own state; use vibe_run_status instead.",
			InputSchema: schema(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"kinds":{"type":"array","items":{"type":"string","enum":["semantic","episodic","correction","preference"]}},"limit":{"type":"integer","minimum":1,"maximum":25}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return searchMemory(deps, raw) },
		},
		{
			Name:        "vibe_memory_propose",
			Description: "Call after establishing something evidence-backed worth remembering across runs. Do not call with a claim that has no cited evidence; it will be rejected.",
			InputSchema: schema(`{"type":"object","required":["kind","content","evidence","sourceType"],"properties":{"kind":{"type":"string","enum":["semantic","episodic","correction","preference"]},"content":{"type":"string"},"evidence":{"type":"array","minItems":1,"items":{"type":"string"}},"sourceType":{"type":"string","enum":["command_result","file_content","ci_api","human_statement","review_comment"]},"sourceRef":{"type":"string"},"confidence":{"type":"number","minimum":0,"maximum":1},"tags":{"type":"array","items":{"type":"string"}}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return proposeMemory(deps, raw) },
		},
		{
			Name: "vibe_run_start",
			Description: "Call to create a run when none exists yet for this objective. Pass the user's objective as goal; derive slug when omitted. " +
				"Use workflow research for literature/experiment loops, delivery (default) for spec/build/ship. Do not call when a run for this slug is already active; use vibe_run_status instead.",
			InputSchema: schema(`{"type":"object","required":["goal"],"properties":{"goal":{"type":"string"},"slug":{"type":"string"},"workflow":{"type":"string","enum":["delivery","research"],"default":"delivery"},"graph":{"type":"string","description":"Advanced override only"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return runStart(deps, raw) },
		},
		{
			Name: "vibe_auto_start",
			Description: "Call to start an unattended auto run from the user's objective. Requires workspace auto opt-in. " +
				"Use workflow research for researcher-delivery, delivery (default) for goal-delivery. Do not call when auto.yaml is unanswered or when a run for this slug already exists.",
			InputSchema: schema(`{"type":"object","required":["goal"],"properties":{"goal":{"type":"string"},"slug":{"type":"string"},"workflow":{"type":"string","enum":["delivery","research"],"default":"delivery"},"taskSource":{"type":"string"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return autoStart(deps, raw) },
		},
		{
			Name:        "vibe_run_status",
			Description: "Call to see the current node, required action, and blockers. Do not call to advance the run; use vibe_checkpoint or vibe_verify instead.",
			InputSchema: schema(`{"type":"object","required":["slug"],"properties":{"slug":{"type":"string"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return runStatus(deps, raw) },
		},
		{
			Name:        "vibe_task_packet",
			Description: "Call to get the next actionable task (acceptance, branch, files) in one call. Do not call before the plan node has run; there is no task list yet.",
			InputSchema: schema(`{"type":"object","required":["slug"],"properties":{"slug":{"type":"string"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return taskPacket(deps, raw) },
		},
		{
			Name: "vibe_repo_map",
			Description: "Call to get a token-budgeted map of the most referenced definitions in the workspace before reading files by hand. " +
				"Do not call when you already have the files you need open; this rebuilds the map each time and is not a cache.",
			InputSchema: schema(`{"type":"object","properties":{"budget":{"type":"integer","minimum":1,"description":"Approximate token budget for the returned map"},"focus":{"type":"string","description":"Path prefix that raises matching files in the ranking without excluding others"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return repoMap(deps, raw) },
		},
		{
			Name: "vibe_experiment_status",
			Description: "Call to read experiment STATUS.md for a slug (running|done|failed) while monitoring a researcher-delivery run. " +
				"Do not call to start or sandbox a GPU job; compute stays on the host or CI.",
			InputSchema: schema(`{"type":"object","required":["slug"],"properties":{"slug":{"type":"string"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return experimentStatus(deps, raw) },
		},
		{
			Name:        "vibe_checkpoint",
			Description: "Call to record evidence (exit code, file assertion, CI response, or human approval) and advance the graph. Do not call for a verifier node's check; call vibe_verify instead.",
			InputSchema: schema(`{"type":"object","required":["slug"],"properties":{"slug":{"type":"string"},"check":{"type":"object","required":["name","passed","source"],"properties":{"name":{"type":"string"},"passed":{"type":"boolean"},"skipped":{"type":"boolean"},"source":{"type":"string","enum":["exit_code","file_assert","ci_api","human_event"]},"ref":{"type":"string"}}},"result":{"type":"object","additionalProperties":{"type":"boolean"}},"blocker":{"type":"string"}}}`),
			Handler:     func(raw json.RawMessage) (any, error) { return runCheckpoint(deps, raw) },
		},
		{
			// No verdict parameter, by design. The tool runs what the workspace's
			// vibe-checks.yaml declares for the current node's check and records
			// what happened. A caller cannot supply the outcome or the command.
			Name:        "vibe_verify",
			Description: "Call at a verifier node to run its declared check and record the real result. Do not call with a verdict already decided; this tool produces its own and takes none as input.",
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
			runSummary := summarize(run)
			runSummary["goal"] = run.Goal
			out["run"] = runSummary
		}
	}
	if store := deps.read(); store != nil {
		hits, err := store.Search(context.Background(), memory.Query{
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
	store := deps.read()
	if store == nil {
		// Nothing stored here yet is an answer, not a fault. Erroring would make
		// a fresh workspace look broken on the first tool call a host makes.
		return map[string]any{"memories": []any{}, "policy": MemoryDisclaimer}, nil
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
	hits, err := store.Search(context.Background(), query)
	if err != nil {
		return nil, err
	}
	return map[string]any{"memories": renderHits(hits), "policy": MemoryDisclaimer}, nil
}

func proposeMemory(deps Deps, raw json.RawMessage) (any, error) {
	if deps.Memory == nil {
		return nil, fmt.Errorf("memory store is not available")
	}
	// A write is what creates the database. Everything before this point can run
	// in a workspace that has never stored anything and leave it as it was.
	store, err := deps.Memory.Write(context.Background())
	if err != nil {
		return nil, err
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

	stored, decision, err := store.Propose(context.Background(), memory.Record{
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
		Slug     string `json:"slug"`
		Goal     string `json:"goal"`
		Workflow string `json:"workflow"`
		Graph    string `json:"graph"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	resolved, err := graphroute.Params{
		Command:       graphroute.CmdGoal,
		Workflow:      graphroute.Workflow(args.Workflow),
		Goal:          args.Goal,
		Slug:          args.Slug,
		GraphOverride: args.Graph,
	}.Resolve()
	if err != nil {
		return nil, err
	}

	result, err := runstart.Start(runstart.Options{
		WorkspaceRoot: deps.WorkspaceRoot,
		ToolkitRoot:   deps.ToolkitRoot,
		Resolved:      resolved,
		Now:           deps.now(),
	})
	if err != nil {
		return nil, err
	}
	deps.Session.Touch(resolved.Slug)
	out := describeGraphRun(deps, result.Run)
	out["goal"] = result.Run.Goal
	out["slug"] = resolved.Slug
	return out, nil
}

func autoStart(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Slug       string `json:"slug"`
		Goal       string `json:"goal"`
		Workflow   string `json:"workflow"`
		TaskSource string `json:"taskSource"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	config, found, err := autoconfig.Load(deps.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	if !found {
		path, writeErr := autoconfig.Write(deps.WorkspaceRoot)
		if writeErr != nil {
			return nil, writeErr
		}
		return nil, fmt.Errorf("workspace has not answered the auto opt-in; wrote %s", path)
	}
	resolved, err := graphroute.Params{
		Command:  graphroute.CmdAuto,
		Workflow: graphroute.Workflow(args.Workflow),
		Goal:     args.Goal,
		Slug:     args.Slug,
	}.Resolve()
	if err != nil {
		return nil, err
	}
	result, err := runstart.Start(runstart.Options{
		WorkspaceRoot: deps.WorkspaceRoot,
		ToolkitRoot:   deps.ToolkitRoot,
		Resolved:      resolved,
		Auto:          true,
		TaskSource:    args.TaskSource,
		TokenBudget:   config.Spec.Budgets.Tokens,
		WallclockSec:  config.Spec.Budgets.WallclockSeconds,
		Now:           deps.now(),
	})
	if err != nil {
		return nil, err
	}
	deps.Session.Touch(resolved.Slug)
	out := describeGraphRun(deps, result.Run)
	out["goal"] = result.Run.Goal
	out["slug"] = resolved.Slug
	out["auto"] = true
	return out, nil
}

func describeGraphRun(deps Deps, run *state.Run) map[string]any {
	loaded, err := graph.LoadByID(graph.DefaultDir(deps.ToolkitRoot), run.GraphID)
	if err != nil {
		return map[string]any{"runId": run.RunID, "node": run.CurrentNode, "graph": run.GraphID}
	}
	return describe(loaded, run)
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
	deps.Session.Touch(args.Slug)
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

	deps.Session.Touch(args.Slug)
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
	deps.Session.NoteListChanged()
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
	deps.Session.Touch(args.Slug)
	if result.Duplicate {
		// A tool call that timed out after the write gets retried. Telling the
		// caller its evidence was already recorded is more useful than either
		// advancing twice or reporting a transition that did not happen.
		out["duplicate"] = true
		out["note"] = "This exact evidence was the last checkpoint recorded, so nothing advanced."
		return out, nil
	}
	deps.Session.NoteListChanged()
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

	if node.Check != "" {
		if check, ok := run.Checks[node.Check]; ok {
			out["currentNodeCheck"] = map[string]any{
				"name": node.Check, "passed": check.Passed, "skipped": check.Skipped,
				"source": string(check.Source), "ref": check.Ref,
			}
		}
	}

	action := map[string]any{
		"kind": string(node.Type), "description": node.Description,
		"relevantTools": relevantToolsFor(node),
	}
	switch node.Type {
	case graph.NodeAgent:
		action["command"] = node.Command
		if run.Flags["auto"] {
			action["completion"] = "Do the work on host compute, then call vibe_checkpoint. Do not ask the human for next steps."
		} else {
			action["completion"] = "Do the work, then call vibe_checkpoint. Agent nodes produce no automatic pass."
		}
	case graph.NodeArtifact:
		action["command"] = node.Command
		action["outputs"] = node.Outputs
		if run.Flags["auto"] {
			action["completion"] = "Write every listed output with no open questions or TBD markers, then call vibe_checkpoint. Settled gate documents let the runtime skip approval gates automatically."
		} else {
			action["completion"] = "Every listed output must exist and be non-empty."
		}
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
		if run.Flags["auto"] {
			if _, ok := auto.GateSpecFor(run.CurrentNode); ok {
				action["completion"] = "On the auto path, call vibe_checkpoint after the gate documents are written; the runtime answers from RESEARCH, PLAN, or TASKS when nothing is open."
				break
			}
		}
		action["completion"] = "Ask the human. Only a recorded human approval advances this node."
	case graph.NodeTerminal:
		action["completion"] = "The run is finished."
	}
	out["requiredAction"] = action
	out["graph"] = loaded.Metadata.ID
	if hint := autoContinueHint(loaded.Metadata.ID, run); hint != "" {
		out["autoContinue"] = hint
	}
	if neighbors, err := loop.New(loaded).Neighbors(run); err == nil && len(neighbors) > 0 {
		out["neighbors"] = neighborPayload(neighbors)
	}
	return out
}

// autoContinueHint tells an auto run's host not to stop for a person mid-pipeline.
func autoContinueHint(graphID string, run *state.Run) string {
	if run == nil || !run.Flags["auto"] {
		return ""
	}
	if run.Status == state.StatusDone || run.Status == state.StatusFailed ||
		run.Status == state.StatusBudgetExceeded {
		return ""
	}
	switch graphID {
	case "researcher-delivery":
		return "Auto research: complete literature, hypothesis, experiment_design, experiment_run, findings, and writeup without asking the human. Call vibe_checkpoint after each artifact and vibe_verify at verifiers until status is done, or stop only when a gate document leaves something open."
	case "goal-delivery":
		return "Auto delivery: complete every node until status is done without asking the human. Call vibe_checkpoint after artifacts; gates skip when SPEC, PLAN, and TASKS have no open markers."
	default:
		return "Auto mode: complete every node until status is done without asking the human except when a gate document leaves items open."
	}
}

func neighborPayload(neighbors []loop.Neighbor) []map[string]any {
	out := make([]map[string]any, 0, len(neighbors))
	for _, nb := range neighbors {
		row := map[string]any{
			"to": nb.To, "via": nb.Via, "toType": nb.ToType,
			"toDescription": nb.ToDescription, "matchesNow": nb.MatchesNow,
			"activePath": nb.ActivePath, "evidenceHint": nb.EvidenceHint,
		}
		if nb.Guard != "" {
			row["guard"] = nb.Guard
			row["negated"] = nb.Negated
			row["guardDescription"] = nb.GuardDescription
			row["guardSource"] = nb.GuardSource
		}
		if len(nb.Resets) > 0 {
			row["resets"] = nb.Resets
		}
		out = append(out, row)
	}
	return out
}

// relevantToolsFor names the MCP tools that matter most at this node.
//
// vibe_checkpoint and vibe_verify still vary by node type: those two error at
// the wrong type, and tools/list hides the one that does not apply.
// Command- and check-specific tools are hints only: research surfaces fetch,
// experiment surfaces STATUS, build surfaces the task packet and repo map.
// tools/list does not hide those hints; narrowing further would drop a tool
// a caller still needs mid-node (for example fetch while reading a check log
// at a verifier).
func relevantToolsFor(node graph.Node) []string {
	switch node.Type {
	case graph.NodeTerminal:
		return nil
	case graph.NodeVerifier:
		out := []string{"vibe_verify"}
		if node.Check == "experiment_done" {
			out = append(out, "vibe_experiment_status")
		}
		if node.Check == "results_acceptable" {
			out = append(out, "vibe_experiment_status")
		}
		return out
	default: // agent, artifact, human_gate
		out := []string{"vibe_checkpoint"}
		switch node.Command {
		case "research", "findings":
			out = append(out, "vibe_fetch")
		case "experiment":
			out = append(out, "vibe_experiment_status")
		case "build", "code-simplify":
			out = append(out, "vibe_task_packet", "vibe_repo_map")
		}
		return out
	}
}

// summarize is the envelope every tool response shares: run identity and
// position, plus a check count rather than the full history. It deliberately
// omits the goal - that text does not change between calls on the same run, so
// repeating it on every status/checkpoint/verify response would spend tokens
// for no new information. bootstrap and runStart, the two calls a caller makes
// before it has necessarily seen the goal, add it back explicitly.
func summarize(run *state.Run) map[string]any {
	out := map[string]any{
		"runId":         run.RunID,
		"slug":          run.Slug,
		"currentNode":   run.CurrentNode,
		"status":        string(run.Status),
		"iteration":     run.Iteration,
		"budget":        run.MaxTransitions,
		"checksSummary": checksSummary(run.Checks),
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

// checksSummary counts a run's recorded checks by outcome, instead of
// serializing every one of them by name on every call. A caller that needs the
// full detail behind one check gets it from currentNodeCheck (describe) or
// from the check/passed/evidence fields checkpoint and verify already return
// for the check they just recorded.
func checksSummary(checks map[string]state.Check) map[string]any {
	summary := map[string]int{"total": len(checks)}
	for _, check := range checks {
		switch {
		case check.Skipped:
			summary["skipped"]++
		case check.Passed:
			summary["passed"]++
		default:
			summary["failed"]++
		}
	}
	return map[string]any{
		"total": summary["total"], "passed": summary["passed"],
		"skipped": summary["skipped"], "failed": summary["failed"],
	}
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

// DefaultFetchBudget matches the CLI's, so the same source costs the same
// whichever surface asked for it.
const DefaultFetchBudget = 4000

// fetchSource returns a document already clipped to a budget.
//
// Clipped here rather than by the caller, because the point of the tool is that
// the untrimmed text never reaches a context window. Handing back everything
// and trusting the reader to stop would be the behaviour this exists to
// replace.
func fetchSource(deps Deps, raw json.RawMessage) (any, error) {
	var args struct {
		Source   string `json:"source"`
		Budget   int    `json:"budget"`
		Refresh  bool   `json:"refresh"`
		ClipFrom string `json:"clipFrom"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("vibe_fetch: %w", err)
	}
	if strings.TrimSpace(args.Source) == "" {
		return nil, fmt.Errorf("vibe_fetch needs a source")
	}
	budget := args.Budget
	if budget <= 0 {
		budget = DefaultFetchBudget
	}
	clipFrom := args.ClipFrom
	if clipFrom == "" {
		clipFrom = "head"
	}
	if clipFrom != "head" && clipFrom != "tail" {
		return nil, fmt.Errorf("vibe_fetch clipFrom must be head or tail, got %q", clipFrom)
	}

	source, err := resolveFetchSource(deps.WorkspaceRoot, args.Source)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	document, cached, err := fetch.Get(ctx, deps.WorkspaceRoot, source, fetch.Options{Refresh: args.Refresh})
	if err != nil {
		return nil, fmt.Errorf("vibe_fetch %s: %w", args.Source, err)
	}

	clipped, omitted := fetch.ClipFrom(document.Text, budget, clipFrom)
	return map[string]any{
		"source":       args.Source,
		"title":        document.Title,
		"text":         clipped,
		"cached":       cached,
		"omittedLines": omitted,
		"tokens":       fetch.EstimateTokens(clipped),
		"clipFrom":     clipFrom,
	}, nil
}

// resolveFetchSource maps check:<slug>:<name> onto the verifier log convention
// RunDir/<name>/<name>.log. Other sources pass through unchanged.
func resolveFetchSource(workspaceRoot, source string) (string, error) {
	const prefix = "check:"
	if !strings.HasPrefix(source, prefix) {
		return source, nil
	}
	rest := strings.TrimPrefix(source, prefix)
	slug, name, ok := strings.Cut(rest, ":")
	if !ok || slug == "" || name == "" {
		return "", fmt.Errorf("vibe_fetch check source must be check:<slug>:<name>, got %q", source)
	}
	runDir := state.RunDir(workspaceRoot, slug)
	if runDir == "" {
		return "", fmt.Errorf("vibe_fetch check:%s:%s: run not started yet", slug, name)
	}
	path := filepath.Join(runDir, name, name+".log")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("vibe_fetch check:%s:%s: not run yet", slug, name)
		}
		return "", fmt.Errorf("vibe_fetch check:%s:%s: %w", slug, name, err)
	}
	return path, nil
}

// fetchTimeout bounds one call. A tool that hangs is worse than one that fails,
// because the model has no way to tell the difference from the inside.
const fetchTimeout = 60 * time.Second
