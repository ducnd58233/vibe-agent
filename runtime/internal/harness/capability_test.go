package harness

import (
	"bytes"
	"encoding/json"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"strings"
	"testing"
)

// hostsWhere returns every host whose dialect satisfies want. Tests select hosts
// by what they can do, not by name, so a host added to the contract table is
// covered by every rule it falls under without a test naming it.
func hostsWhere(t *testing.T, want func(HostContract) bool) []Client {
	t.Helper()
	var out []Client
	for _, contract := range HostContracts() {
		if want(contract) {
			out = append(out, contract.Client)
		}
	}
	if len(out) == 0 {
		t.Skip("no host in the contract table has this capability")
	}
	return out
}

func runWithOutput(t *testing.T, req Request) (string, error) {
	t.Helper()
	if req.ToolkitRoot == "" {
		req.ToolkitRoot = toolkitRoot
	}
	var out bytes.Buffer
	err := Run(req, &out)
	return out.String(), err
}

// A host that appends hook stdout to the context must get the text itself; a
// JSON envelope would arrive in front of the model as literal JSON.
func TestPlainContextHostsGetTextNotJSON(t *testing.T) {
	for _, client := range hostsWhere(t, func(c HostContract) bool { return c.Dialect.Context == ContextPlain }) {
		root := workspaceWithRun(t)
		out, err := runWithOutput(t, Request{
			Event: EventUserPromptSubmit, Client: client, WorkspaceRoot: root,
			Stdin: strings.NewReader(`{"session_id":"s1","prompt":"status"}`),
		})
		if err != nil {
			t.Fatalf("%s: %v", client, err)
		}
		if !strings.Contains(out, "Run demo is at node") || json.Valid([]byte(strings.TrimSpace(out))) {
			t.Errorf("%s: want plain text context, got %q", client, out)
		}
	}
}

// A host whose stop hook reads only the exit status must be refused through it.
func TestExitStopHostsAreBlockedThroughTheExitStatus(t *testing.T) {
	for _, client := range hostsWhere(t, func(c HostContract) bool { return c.Dialect.StopBlock == StopExit }) {
		root := workspaceWithRun(t, func(r *state.Run) { r.CurrentNode = "test" })
		out, err := runWithOutput(t, Request{Event: EventStop, Client: client, WorkspaceRoot: root})
		var blocked *BlockError
		if !asBlock(err, &blocked) || blocked.Reason == "" {
			t.Errorf("%s: stop was not refused through the exit status: err=%v out=%q", client, err, out)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("%s: a stdout body was written that the host does not read: %q", client, out)
		}
	}
}

// A host measured to fail open on everything but the blocking status must get
// the refusal on both channels.
func TestFailOpenHostsGetTheRefusalOnBothChannels(t *testing.T) {
	for _, client := range hostsWhere(t, func(c HostContract) bool { return c.Dialect.RefusalExits }) {
		root := workspaceWithRun(t)
		out, err := runWithOutput(t, Request{
			Event: EventPreToolUse, Client: client, WorkspaceRoot: root,
			Stdin: strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}`),
		})
		var blocked *BlockError
		if !asBlock(err, &blocked) {
			t.Errorf("%s: no blocking exit: %v", client, err)
		}
		if !strings.Contains(out, "deny") {
			t.Errorf("%s: no JSON deny on stdout: %q", client, out)
		}
	}
}

// Every host's own write tools reach the guards. A name missing from a host's
// vocabulary is a tool whose writes are never scanned.
func TestEveryHostsWriteToolsAreRecognised(t *testing.T) {
	for _, contract := range HostContracts() {
		if len(contract.Tools.Writes) == 0 {
			t.Errorf("%s declares no write tools; its edits would never be scanned", contract.Client)
		}
		for _, tool := range contract.Tools.Writes {
			if !isFileWrite(contract.Client, tool) {
				t.Errorf("%s: %s not recognised as a write", contract.Client, tool)
			}
		}
		if isFileWrite(contract.Client, "Read") || isFileWrite(contract.Client, "read") {
			t.Errorf("%s: a read was treated as a write", contract.Client)
		}
	}
}

// camelCase stdin (conversationId, toolCall.name/args) normalises to the same
// fields the rest of the package reads.
func TestCamelCasePayloadsNormalise(t *testing.T) {
	body := readPayload(strings.NewReader(`{"conversationId":"c-1","transcriptPath":"/t.jsonl",` +
		`"toolCall":{"name":"run_command","args":{"CommandLine":"go test ./..."}}}`))
	if body.sessionKey() != "c-1" || body.ToolName != "run_command" || body.shellCommand() != "go test ./..." {
		t.Errorf("normalised to session=%q tool=%q command=%q", body.sessionKey(), body.ToolName, body.shellCommand())
	}
}
