package harness

import (
	"strings"
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

// A host that cannot inject at the prompt learns which node its run is at only
// from a tool call's reply. Before this it learned nowhere,
// while commands/goal.md claimed injection happened on every prompt.
func TestAHostWithoutPromptInjectionLearnsTheNodeFromAToolCall(t *testing.T) {
	root := workspaceWithRun(t)

	output := invoke(t, Request{
		Event: EventPostToolUse, Client: hostWhere(t, remindsAfterToolUse), WorkspaceRoot: root,
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
	})

	if !strings.Contains(output, "additional_context") {
		t.Fatalf("the host was told nothing: %s", output)
	}
	if !strings.Contains(output, "demo") || !strings.Contains(output, "node test") {
		t.Errorf("the node did not reach the host: %s", output)
	}
}

// Once per change, not once per tool call. A run sitting at one node for twenty
// edits has nothing new to say, and repeating it teaches a reader to skip the
// line that matters when it finally changes.
func TestTheNodeIsNotRepeatedOnEveryToolCall(t *testing.T) {
	root := workspaceWithRun(t)
	call := func() string {
		return invoke(t, Request{
			Event: EventPostToolUse, Client: hostWhere(t, remindsAfterToolUse), WorkspaceRoot: root,
			Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
		})
	}

	if first := call(); !strings.Contains(first, "node test") {
		t.Fatalf("the first call said nothing: %s", first)
	}
	if second := call(); second != "" {
		t.Errorf("the same node was announced twice: %s", second)
	}
}

// A node change is the thing worth interrupting for, so it has to get through
// after the quiet period above.
func TestANodeChangeIsAnnouncedAfterAToolCall(t *testing.T) {
	root := workspaceWithRun(t)
	call := func() string {
		return invoke(t, Request{
			Event: EventPostToolUse, Client: hostWhere(t, remindsAfterToolUse), WorkspaceRoot: root,
			Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
		})
	}
	call()

	run, err := state.Load(state.ManifestPath(root, "demo"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	run.CurrentNode = "review"
	testutil.EnsureRunIndex(t, root, "demo")
	if err := state.Save(state.ManifestPath(root, "demo"), run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if output := call(); !strings.Contains(output, "node review") {
		t.Errorf("the node change did not reach the host: %s", output)
	}
}

// Every other host injects on every prompt and must not also get this, or the
// same fact arrives twice by two routes.
func TestHostsThatInjectAtThePromptDoNotAlsoGetTheToolCallReminder(t *testing.T) {
	for _, client := range Clients() {
		if contract, _ := HostContractFor(client); contract.Dialect.ToolUseNodeReminder {
			continue
		}
		root := workspaceWithRun(t)
		output := invoke(t, Request{
			Event: EventPostToolUse, Client: client, WorkspaceRoot: root,
			Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
		})
		if strings.Contains(output, "no prompt-time injection") {
			t.Errorf("%s got the tool-call reminder as well as its own prompt injection: %s", client, output)
		}
	}
}

// With no run there is nothing to report, and with several, naming one would be
// choosing which goal the person is working on.
func TestNoNodeReminderWithoutExactlyOneRun(t *testing.T) {
	output := invoke(t, Request{
		Event: EventPostToolUse, Client: hostWhere(t, remindsAfterToolUse), WorkspaceRoot: t.TempDir(),
		Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`),
	})
	if output != "" {
		t.Errorf("a workspace with no run produced context: %s", output)
	}
}
