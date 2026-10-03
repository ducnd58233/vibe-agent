package harness

import (
	"bytes"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"
)

// These tests read the contract table rather than repeating its values, and
// they run for every host.
//
// A test asserting `agent_message` against a literal proves the code matches
// the test. Asserting it against the table proves the code matches the recorded
// vendor contract, and makes the table the thing to correct when a host
// changes. The defect that motivated this, a camelCase key where a host reads
// snake_case, would have passed any test written from the same misreading as
// the code.

// flatKeys names every field of an envelope the way OutputKeys does: a nested
// object's fields as "outer.inner".
func flatKeys(body map[string]any) []string {
	var keys []string
	for key, value := range body {
		if nested, ok := value.(map[string]any); ok {
			for inner := range nested {
				keys = append(keys, key+"."+inner)
			}
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// recordedKeys is every output key the host's contract records for the events
// that match.
func recordedKeys(contract HostContract, match func(EventContract) bool) []string {
	var keys []string
	for _, event := range contract.Events {
		if match(event) {
			keys = append(keys, event.OutputKeys...)
		}
	}
	return keys
}

// Every key a host is sent must be one its contract records it reading. A key
// outside that set is silently discarded, which is how a refusal once arrived
// with no reason attached.
func TestEveryEnvelopeUsesOnlyKeysTheHostsContractRecords(t *testing.T) {
	const reason = "push to main is refused"
	for _, contract := range HostContracts() {
		d := contract.Dialect
		t.Run(string(contract.Client), func(t *testing.T) {
			if body := refusalBody(d, reason); body != nil {
				allowed := recordedKeys(contract, func(e EventContract) bool { return e.Event == EventPreToolUse && e.CanRefuse })
				for _, key := range flatKeys(roundTrip(t, body)) {
					if !slices.Contains(allowed, key) {
						t.Errorf("refusal sends %q; the contract records %v", key, allowed)
					}
				}
			}
			if body := contextBody(d, "SessionStart", "context"); body != nil {
				allowed := recordedKeys(contract, func(e EventContract) bool { return e.CanInject })
				for _, key := range flatKeys(roundTrip(t, body)) {
					if !slices.Contains(allowed, key) {
						t.Errorf("context sends %q; the contract records %v", key, allowed)
					}
				}
			}
			if body := stopBody(d, reason); body != nil {
				allowed := recordedKeys(contract, func(e EventContract) bool { return e.Event == EventStop })
				for _, key := range flatKeys(roundTrip(t, body)) {
					if !slices.Contains(allowed, key) {
						t.Errorf("stop sends %q; the contract records %v", key, allowed)
					}
				}
			}
		})
	}
}

// The reason has to survive the trip, and a refusal is always a deny. A
// refusal honoured while its explanation is discarded leaves the agent blocked,
// told nothing, and retrying; an "ask" is not valid on every event one refusal
// shape answers.
func TestEveryRefusalDeniesAndCarriesTheReason(t *testing.T) {
	const reason = "push to main is refused"
	for _, contract := range HostContracts() {
		body := refusalBody(contract.Dialect, reason)
		if body == nil {
			continue
		}
		raw, _ := json.Marshal(body)
		text := string(raw)
		if !strings.Contains(text, reason) {
			t.Errorf("%s: the reason did not reach the agent: %s", contract.Client, text)
		}
		if !strings.Contains(text, `"deny"`) || strings.Contains(text, `"ask"`) {
			t.Errorf("%s: a refusal must be a deny and never an ask: %s", contract.Client, text)
		}
	}
}

// A host that decides through JSON must get the verdict as JSON, not as an
// error it cannot read, end to end through the hook.
func TestJSONRefusingHostsGetADecisionNotAnError(t *testing.T) {
	for _, contract := range HostContracts() {
		d := contract.Dialect
		if d.Refusal == RefuseExit || d.RefusalExits {
			continue
		}
		var out bytes.Buffer
		err := Run(Request{
			Event: EventPreToolUse, Client: contract.Client, WorkspaceRoot: workspaceWithRun(t),
			ToolkitRoot: toolkitRoot,
			Stdin:       strings.NewReader(`{"tool_name":"Bash","command":"git push origin main","tool_input":{"command":"git push origin main"}}`),
		}, &out)
		if err != nil {
			t.Errorf("%s: the gate returned an error the host cannot read: %v", contract.Client, err)
			continue
		}
		if !strings.Contains(out.String(), `"deny"`) {
			t.Errorf("%s: was not told to deny: %s", contract.Client, out.String())
		}
	}
}
