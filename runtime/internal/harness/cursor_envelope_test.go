package harness

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// These tests read the contract table rather than repeating its values.
//
// A test asserting `agent_message` against a literal proves the code matches
// the test. Asserting it against the table proves the code matches the recorded
// vendor contract, and makes the table the thing to correct when a host
// changes. That difference is the whole reason the table exists: the defect
// being fixed here, agentMessage where Cursor reads agent_message, would have
// passed any test written from the same misreading as the code.

// cursorDenyPayload produces the refusal Cursor receives.
func cursorDenyPayload(t *testing.T) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := deliverBlock(Request{Client: ClientCursor},
		&BlockError{Reason: "push to main is refused"}, &out); err != nil {
		t.Fatalf("deliverBlock: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("Cursor payload is not JSON: %v: %s", err, out.String())
	}
	return decoded
}

// Every key Cursor is sent must be one the contract records it reading. A key
// outside that set is silently discarded, which is exactly how a refusal
// arrived with no reason attached.
func TestCursorDenyUsesOnlyKeysTheContractRecords(t *testing.T) {
	contract, ok := HostContractFor(ClientCursor)
	if !ok {
		t.Fatal("no Cursor contract")
	}
	event, ok := contract.EventFor("beforeShellExecution")
	if !ok {
		t.Fatal("the contract records no beforeShellExecution row")
	}

	for key := range cursorDenyPayload(t) {
		if !slices.Contains(event.OutputKeys, key) {
			t.Errorf("Cursor is sent %q, which the contract does not record it reading; "+
				"recorded keys are %v", key, event.OutputKeys)
		}
	}
}

// The reason has to survive the trip. The refusal Cursor honoured while
// discarding its explanation is the defect that made this worth a test: the
// agent was blocked, told nothing, and retried.
func TestCursorDenyCarriesTheReason(t *testing.T) {
	payload := cursorDenyPayload(t)

	message, ok := payload["agent_message"].(string)
	if !ok {
		t.Fatalf("no agent_message in %v", payload)
	}
	if !strings.Contains(message, "push to main is refused") {
		t.Errorf("the reason did not reach the agent: %q", message)
	}
	if _, ok := payload["user_message"].(string); !ok {
		t.Errorf("no user_message for the person to read: %v", payload)
	}
}

// preToolUse accepts allow and deny; beforeShellExecution also accepts ask.
// One branch answers both events, which is only correct while the value it
// sends is one they share. If a future change wants "ask", it has to split the
// branch by event first, and this is the test that will say so.
func TestCursorNeverAsks(t *testing.T) {
	permission, ok := cursorDenyPayload(t)["permission"].(string)
	if !ok {
		t.Fatal("no permission field")
	}
	if permission != "deny" {
		t.Errorf("permission is %q; preToolUse accepts only allow and deny, so a single "+
			"branch may send neither ask nor anything else", permission)
	}
}

// The keys the runtime sends on Cursor's other events, checked the same way.
// Casing drifted once and the cost was invisible, so it is worth pinning
// wherever this package writes a Cursor payload at all.
func TestCursorContextAndStopUseContractKeys(t *testing.T) {
	contract, _ := HostContractFor(ClientCursor)

	for _, host := range []struct {
		hostKey string
		produce func(io *bytes.Buffer) error
	}{
		{"sessionStart", func(out *bytes.Buffer) error {
			return emitContext(out, ClientCursor, "SessionStart", "some context")
		}},
		{"stop", func(out *bytes.Buffer) error {
			return write(out, map[string]any{"followup_message": "keep going"})
		}},
	} {
		t.Run(host.hostKey, func(t *testing.T) {
			event, ok := contract.EventFor(host.hostKey)
			if !ok {
				t.Fatalf("the contract records no %s row", host.hostKey)
			}
			var out bytes.Buffer
			if err := host.produce(&out); err != nil {
				t.Fatalf("produce: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
				t.Fatalf("not JSON: %v: %s", err, out.String())
			}
			for key := range decoded {
				if !slices.Contains(event.OutputKeys, key) {
					t.Errorf("%s is sent %q, not in the contract's %v", host.hostKey, key, event.OutputKeys)
				}
			}
		})
	}
}
