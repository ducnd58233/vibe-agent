package harness

import (
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"strconv"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/agentstate"
)

func promptIn(t *testing.T, root, session, prompt string) string {
	t.Helper()
	return invoke(t, Request{
		Event: EventUserPromptSubmit, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"session_id":"` + session + `","prompt":"` + prompt + `"}`),
	})
}

// The point of the ledger: a session already holds the run line and the memory,
// so the second prompt pays for neither.
func TestARepeatedPromptIsNotAnsweredWithTheSameContext(t *testing.T) {
	root := workspaceWithRun(t)
	seedMemory(t, root, "the runtime module builds with CGO disabled")

	first := promptIn(t, root, "s1", "why is CGO turned off here")
	if !strings.Contains(first, "CGO disabled") || !strings.Contains(first, "demo") {
		t.Fatalf("first prompt missed its context: %s", first)
	}
	second := promptIn(t, root, "s1", "why is CGO turned off here")
	if second != "" {
		t.Errorf("an unchanged context was sent again: %s", second)
	}
}

func TestContextIsSentAgainWhenTheRunMoves(t *testing.T) {
	root := workspaceWithRun(t)
	if out := promptIn(t, root, "s1", "status please"); !strings.Contains(out, "node test") {
		t.Fatalf("first prompt: %s", out)
	}
	if out := promptIn(t, root, "s1", "status please"); out != "" {
		t.Fatalf("second prompt repeated itself: %s", out)
	}
	root2 := workspaceWithRun(t, func(r *state.Run) { r.CurrentNode = "build" })
	if out := promptIn(t, root2, "s1", "status please"); !strings.Contains(out, "node build") {
		t.Errorf("a different node was suppressed: %s", out)
	}
}

// A host may compact and lose what was injected, so nothing is trusted forever.
func TestContextIsRefreshedAfterTheWindow(t *testing.T) {
	root := workspaceWithRun(t)
	if out := promptIn(t, root, "s1", "go"); out == "" {
		t.Fatal("first prompt was empty")
	}
	for i := 1; i < injectRefresh; i++ {
		if out := promptIn(t, root, "s1", "go "+strconv.Itoa(i)); out != "" {
			t.Fatalf("prompt %d repeated context inside the window: %s", i+1, out)
		}
	}
	if out := promptIn(t, root, "s1", "go again"); !strings.Contains(out, "demo") {
		t.Errorf("context was not refreshed after %d prompts: %q", injectRefresh, out)
	}
}

func TestSessionStartResetsWhatTheSessionWasTold(t *testing.T) {
	root := workspaceWithRun(t)
	promptIn(t, root, "s1", "hello")
	if out := promptIn(t, root, "s1", "hello"); out != "" {
		t.Fatalf("setup: second prompt was not suppressed: %s", out)
	}
	// Compaction re-fires SessionStart; whatever was delivered is gone.
	invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"session_id":"s1","source":"compact"}`),
	})
	if out := promptIn(t, root, "s1", "hello"); !strings.Contains(out, "demo") {
		t.Errorf("context was not re-sent after a session start: %q", out)
	}
}

func TestSessionStartMemoriesAreNotRepeatedOnTheFirstPrompt(t *testing.T) {
	root := t.TempDir()
	seedMemory(t, root, "the runtime module builds with CGO disabled")
	start := invoke(t, Request{
		Event: EventSessionStart, Client: ClientClaude, WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"session_id":"s1","source":"startup"}`),
	})
	if !strings.Contains(start, "CGO disabled") {
		t.Fatalf("session start did not carry the memory: %s", start)
	}
	if out := promptIn(t, root, "s1", "why is CGO turned off here"); strings.Contains(out, "CGO disabled") {
		t.Errorf("a memory delivered at session start was delivered again: %s", out)
	}
}

func TestSessionsDoNotSuppressEachOther(t *testing.T) {
	root := workspaceWithRun(t)
	promptIn(t, root, "s1", "hello")
	if out := promptIn(t, root, "s2", "hello"); !strings.Contains(out, "demo") {
		t.Errorf("a second conversation inherited the first's ledger: %q", out)
	}
}

func TestANewMemoryTakesTheSlotOfOneAlreadyDelivered(t *testing.T) {
	root := t.TempDir()
	seedMemory(t, root, "the runtime module builds with CGO disabled everywhere")
	first := promptIn(t, root, "s1", "CGO builds")
	if !strings.Contains(first, "CGO disabled everywhere") {
		t.Fatalf("first: %s", first)
	}
	seedMemory(t, root, "linux release builds also pass CGO flags through the makefile")
	second := promptIn(t, root, "s1", "CGO builds")
	if strings.Contains(second, "CGO disabled everywhere") || !strings.Contains(second, "makefile") {
		t.Errorf("second prompt should carry only the new memory: %s", second)
	}
}

// The ledger writes only once it has something to remember. (Session logging
// has its own storage and is not what this checks.)
func TestNothingToInjectWritesNoLedger(t *testing.T) {
	root := t.TempDir()
	if out := promptIn(t, root, "s1", "hello"); out != "" {
		t.Fatalf("unexpected output: %s", out)
	}
	if _, ok, _ := agentstate.Get(t.Context(), root, injectNamespace, "claude:s1"); ok {
		t.Error("an empty prompt wrote a ledger")
	}
}
