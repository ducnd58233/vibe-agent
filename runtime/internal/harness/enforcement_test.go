package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// runHook is invoke's sibling for the cases where the error is the point.
func runHook(t *testing.T, req Request) error {
	t.Helper()
	if req.Stdin == nil {
		req.Stdin = strings.NewReader("{}")
	}
	if req.ToolkitRoot == "" {
		req.ToolkitRoot = toolkitRoot
	}
	var out bytes.Buffer
	return Run(req, &out)
}

func asBlock(err error, target **BlockError) bool {
	return errors.As(err, target)
}

// jsonPath builds a path that survives being embedded in a JSON string, which
// on Windows means forward slashes rather than escaped backslashes.
func jsonPath(root string, parts ...string) string {
	return filepath.ToSlash(filepath.Join(append([]string{root}, parts...)...))
}

// seedMemory puts one confirmed memory in a workspace, the way a real run would
// have: proposed against evidence, then confirmed by a command result.
func seedMemory(t *testing.T, root, content string) {
	t.Helper()
	store, err := memory.Open(t.Context(), root)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := t.Context()
	record, decision, err := store.Propose(ctx, memory.Record{
		WorkspaceID: root,
		Kind:        memory.KindSemantic,
		Content:     content,
		Confidence:  0.9,
		SourceType:  memory.SourceCommandResult,
		Evidence:    []string{"go test ./... exited 0 against the runtime module"},
	}, at())
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if decision.Verdict == memory.VerdictReject {
		t.Fatalf("seed memory rejected: %s", decision.Reason)
	}
	if _, err := store.Confirm(ctx, record.ID, memory.SourceCommandResult, "events.ndjson#1", at()); err != nil {
		t.Fatalf("confirm: %v", err)
	}
}

func decode(t *testing.T, output string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(output), &body); err != nil {
		t.Fatalf("output is not JSON: %v: %s", err, output)
	}
	return body
}

func hookSpecific(t *testing.T, output string) map[string]any {
	t.Helper()
	specific, ok := decode(t, output)["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("no hookSpecificOutput: %s", output)
	}
	return specific
}

// --- Change 1: Stop refuses to let a run be abandoned mid-graph ---------------

func TestStopBlocksWhenARunIsStillRunning(t *testing.T) {
	output := invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: workspaceWithRun(t),
	})
	body := decode(t, output)
	if body["decision"] != "block" {
		t.Errorf("stop did not block an unfinished run: %s", output)
	}
	if reason, _ := body["reason"].(string); !strings.Contains(reason, "demo") {
		t.Errorf("block reason does not name the run: %s", output)
	}
}

// The one rule that keeps a blocking Stop hook from becoming an infinite loop.
func TestStopDoesNotBlockWhenItIsAlreadyBlocking(t *testing.T) {
	output := invoke(t, Request{
		Event: EventStop, Client: ClientClaude, WorkspaceRoot: workspaceWithRun(t),
		Stdin: strings.NewReader(`{"stop_hook_active":true}`),
	})
	if decode(t, output)["decision"] == "block" {
		t.Errorf("stop blocked while already blocking, which loops forever: %s", output)
	}
}

// A human gate cannot be satisfied by the model, so blocking there would spin.
func TestStopDoesNotBlockARunAwaitingAHuman(t *testing.T) {
	root := workspaceWithRun(t, func(run *state.Run) { run.Status = state.StatusAwaitingHuman })
	output := invoke(t, Request{Event: EventStop, Client: ClientClaude, WorkspaceRoot: root})
	if decode(t, output)["decision"] == "block" {
		t.Errorf("stop blocked a run that is waiting on a person: %s", output)
	}
}

// Three cycles on the same blocker is the stop rule. Past it, the run is over.
func TestStopDoesNotBlockAnExhaustedRun(t *testing.T) {
	root := workspaceWithRun(t, func(run *state.Run) {
		run.Blockers = []state.Blocker{{
			Node: "test", Reason: "flaky integration suite",
			Attempts: loop.MaxBlockerAttempts, At: at(),
		}}
	})
	output := invoke(t, Request{Event: EventStop, Client: ClientClaude, WorkspaceRoot: root})
	if decode(t, output)["decision"] == "block" {
		t.Errorf("stop blocked a run that already hit the blocker cap: %s", output)
	}
}

func TestCursorStopContinuesWithAFollowupMessage(t *testing.T) {
	output := invoke(t, Request{
		Event: EventStop, Client: ClientCursor, WorkspaceRoot: workspaceWithRun(t),
	})
	body := decode(t, output)
	if followup, _ := body["followup_message"].(string); !strings.Contains(followup, "demo") {
		t.Errorf("Cursor stop did not send a followup message: %s", output)
	}
}

// --- Change 2: memory reaches the model without the model asking -------------

func TestSessionStartInjectsConfirmedMemory(t *testing.T) {
	root := t.TempDir()
	seedMemory(t, root, "the runtime module builds with CGO disabled")

	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: root,
	})
	if !strings.Contains(output, "CGO disabled") {
		t.Errorf("session start did not inject stored memory: %s", output)
	}
	if !strings.Contains(output, "source of truth") {
		t.Error("injected memory is not labelled as supporting context")
	}
}

func TestPromptSubmitInjectsMemoryMatchingThePrompt(t *testing.T) {
	root := t.TempDir()
	seedMemory(t, root, "the runtime module builds with CGO disabled")

	output := invoke(t, Request{
		Event: EventUserPromptSubmit, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"user_prompt":"why is CGO turned off here"}`),
	})
	if !strings.Contains(output, "CGO disabled") {
		t.Errorf("prompt submit did not retrieve matching memory: %s", output)
	}
}

// The keyword gate that used to sit here meant an ordinary prompt got no run
// context at all, which is the failure this whole change exists to fix.
func TestPromptSubmitSurfacesTheRunOnAnyPrompt(t *testing.T) {
	output := invoke(t, Request{
		Event: EventUserPromptSubmit, Client: ClientClaude,
		WorkspaceRoot: workspaceWithRun(t),
		Stdin:         strings.NewReader(`{"user_prompt":"explain this regex"}`),
	})
	if !strings.Contains(output, "demo") {
		t.Errorf("prompt submit stayed quiet while a run was active: %s", output)
	}
}

// Claude Code sends user_prompt; older builds sent prompt. Both must work.
func TestPromptSubmitReadsEitherPromptField(t *testing.T) {
	root := t.TempDir()
	seedMemory(t, root, "the runtime module builds with CGO disabled")

	for _, field := range []string{"prompt", "user_prompt"} {
		output := invoke(t, Request{
			Event: EventUserPromptSubmit, Client: ClientClaude, WorkspaceRoot: root,
			Stdin: strings.NewReader(`{"` + field + `":"why is CGO turned off here"}`),
		})
		if !strings.Contains(output, "CGO disabled") {
			t.Errorf("field %q was not read: %s", field, output)
		}
	}
}

// --- Change 4: the manifest stops being editable by hand ---------------------

func TestPreToolUseDeniesEditingAManifest(t *testing.T) {
	root := workspaceWithRun(t)
	err := runHook(t, Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Edit","tool_input":{"file_path":"` +
			jsonPath(root, "tmp", "demo", "manifest.json") + `"}}`),
	})
	var blocked *BlockError
	if !asBlock(err, &blocked) {
		t.Fatalf("editing a manifest was allowed: %v", err)
	}
	if !strings.Contains(blocked.Reason, "checkpoint") {
		t.Errorf("the refusal does not name the command that does this properly: %s", blocked.Reason)
	}
}

func TestPreToolUseDeniesAppendingToAnEventLog(t *testing.T) {
	root := workspaceWithRun(t)
	err := runHook(t, Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"echo hi >> tmp/demo/events.ndjson"}}`),
	})
	var blocked *BlockError
	if !asBlock(err, &blocked) {
		t.Fatalf("appending to the event log was allowed: %v", err)
	}
}

func TestPreToolUseAllowsOrdinaryEdits(t *testing.T) {
	root := workspaceWithRun(t)
	err := runHook(t, Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Edit","tool_input":{"file_path":"` +
			jsonPath(root, "runtime", "cmd", "main.go") + `"}}`),
	})
	if err != nil {
		t.Errorf("an ordinary edit was refused: %v", err)
	}
}

// Without a run there is no state to protect, and blocking would break every
// workspace that uses the toolkit without starting a state.
func TestPreToolUseAllowsManifestEditsWithNoRun(t *testing.T) {
	root := t.TempDir()
	err := runHook(t, Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"` +
			jsonPath(root, "tmp", "demo", "manifest.json") + `"}}`),
	})
	if err != nil {
		t.Errorf("a manifest write was refused with no active run: %v", err)
	}
}

// --- Change 5: a resumed session starts on the run, not from scratch ---------

func TestSessionStartSteersASingleActiveRun(t *testing.T) {
	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: workspaceWithRun(t),
	})
	message, _ := hookSpecific(t, output)["initialUserMessage"].(string)
	if !strings.Contains(message, "demo") {
		t.Errorf("session start did not steer the session onto the active run: %s", output)
	}
}

// Compaction re-fires SessionStart mid-session. Steering there would hijack the
// conversation the person is already having.
func TestSessionStartDoesNotSteerAfterCompaction(t *testing.T) {
	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: workspaceWithRun(t),
		Stdin: strings.NewReader(`{"source":"compact"}`),
	})
	if hookSpecific(t, output)["initialUserMessage"] != nil {
		t.Errorf("session start steered a compaction restart: %s", output)
	}
}

func TestSessionStartDoesNotSteerWithNoRun(t *testing.T) {
	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: t.TempDir(),
	})
	if hookSpecific(t, output)["initialUserMessage"] != nil {
		t.Errorf("session start invented work with no active run: %s", output)
	}
}

// --- Change 6: Cursor gets the fields Cursor documents ----------------------

func TestCursorSessionStartUsesAdditionalContext(t *testing.T) {
	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientCursor, WorkspaceRoot: t.TempDir(),
	})
	if decode(t, output)["additional_context"] == nil {
		t.Errorf("Cursor session start does not use additional_context: %s", output)
	}
}

// --- Change 3: tool use lands in the journal ---------------------------------

func TestPostToolUseJournalsAgainstTheActiveRun(t *testing.T) {
	root := workspaceWithRun(t)
	invoke(t, Request{
		Event: EventPostToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"go test ./..."},` +
			`"tool_response":{"stdout":"ok","exit_code":0}}`),
	})
	events, err := state.ReadEvents(state.EventLogPath(root, "demo"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("post-tool-use recorded nothing in the journal")
	}
	last := events[len(events)-1]
	if last.Type != "tool_use" {
		t.Errorf("journal entry has type %q, want tool_use", last.Type)
	}
	if !strings.Contains(string(last.Payload), "go test") {
		t.Errorf("journal entry does not record the command: %s", last.Payload)
	}
}

// A host-reported exit code is the same provenance the manifest accepts for a
// check, so the memory it produces is confirmed rather than left proposed. A
// proposed memory is one retrieval filters out: written and never readable.
func TestPostToolUseRecordsAFailedCommand(t *testing.T) {
	root := workspaceWithRun(t)
	failingCommand(t, root)

	store, err := memory.Open(t.Context(), root)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	defer func() { _ = store.Close() }()
	records, err := store.List(t.Context(), root)
	if err != nil {
		t.Fatalf("list memories: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want one memory, got %d", len(records))
	}
	record := records[0]
	if record.Status != memory.StatusConfirmed {
		t.Errorf("memory is %s, so retrieval will never return it", record.Status)
	}
	if !strings.Contains(record.Content, "go build") {
		t.Errorf("the memory does not name the command: %q", record.Content)
	}
	if record.ExpiresAt == nil {
		t.Error("a command failure was stored without a shelf life")
	}
	if record.SourceType != memory.SourceCommandResult {
		t.Errorf("provenance is %q, want command_result", record.SourceType)
	}
}

// The whole point of writing a memory is that a later session reads it without
// being asked to.
func TestAFailureRecordedNowIsRetrievedNextSession(t *testing.T) {
	root := workspaceWithRun(t)
	failingCommand(t, root)

	output := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: root,
	})
	if !strings.Contains(output, "go build") {
		t.Errorf("the recorded failure did not reach the next session: %s", output)
	}
}

func failingCommand(t *testing.T, root string) {
	t.Helper()
	invoke(t, Request{
		Event: EventPostToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"go build ./..."},` +
			`"tool_response":{"exit_code":2,"stderr":"undefined: EventPostToolUse"}}`),
	})
}

// A command that succeeded is not a fact worth remembering, and one whose
// outcome the host did not report is not a fact at all.
func TestPostToolUseProposesNothingWithoutAFailure(t *testing.T) {
	for _, response := range []string{
		`{"exit_code":0}`,
		`{"stdout":"ok"}`,
		`"not an object"`,
	} {
		root := workspaceWithRun(t)
		invoke(t, Request{
			Event: EventPostToolUse, Client: ClientClaude, WorkspaceRoot: root,
			Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"go build ./..."},` +
				`"tool_response":` + response + `}`),
		})
		if _, err := os.Stat(memory.DBPath(root)); err == nil {
			t.Errorf("response %s created a memory database", response)
		}
	}
}

func TestPostToolUseStaysQuietWithNoRun(t *testing.T) {
	output := invoke(t, Request{
		Event: EventPostToolUse, Client: ClientClaude, WorkspaceRoot: t.TempDir(),
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
	})
	if output != "" {
		t.Errorf("post-tool-use spoke with no active run: %s", output)
	}
}

// A hook that fails a session over its own bookkeeping is worse than one that
// records nothing.
func TestPostToolUseSurvivesAnUnreadableResponse(t *testing.T) {
	root := workspaceWithRun(t)
	if err := runHook(t, Request{
		Event: EventPostToolUse, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_response":"not an object"}`),
	}); err != nil {
		t.Errorf("post-tool-use failed on an unexpected response shape: %v", err)
	}
}
