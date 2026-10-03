package harness

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func roundTrip(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestContextBodyShapes(t *testing.T) {
	nested := map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "E", "additionalContext": "hi"}}
	for shape, want := range map[ContextShape]map[string]any{
		ContextNested: nested,
		ContextFlat:   {"additional_context": "hi"},
		ContextSteps:  {"injectSteps": []any{map[string]any{"ephemeralMessage": "hi"}}},
	} {
		got := roundTrip(t, contextBody(Dialect{Context: shape}, "E", "hi"))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("context %q = %v, want %v", shape, got, want)
		}
	}
}

func TestRefusalBodyShapes(t *testing.T) {
	if refusalBody(Dialect{Refusal: RefuseExit}, "r") != nil {
		t.Error("an exit-code host must get no body: the refusal travels as an error")
	}
	cases := map[RefusalShape]string{
		RefusePermission:       "permission",
		RefusePermissionReason: "permission",
		RefuseDecision:         "decision",
		RefuseHookSpecific:     "hookSpecificOutput",
	}
	for shape, key := range cases {
		body := roundTrip(t, refusalBody(Dialect{Refusal: shape}, "why"))
		if _, ok := body[key]; !ok {
			t.Errorf("refusal %q lacks %q: %v", shape, key, body)
		}
	}
	// Every shape that reports a refusal reports a deny, never an ask: one
	// answer has to be valid for every event that can refuse.
	for shape := range cases {
		raw, _ := json.Marshal(refusalBody(Dialect{Refusal: shape}, "why"))
		if !strings.Contains(string(raw), "deny") {
			t.Errorf("refusal %q does not deny: %s", shape, raw)
		}
	}
}

func TestStopAndPostToolShapes(t *testing.T) {
	if got := roundTrip(t, stopBody(Dialect{}, "r")); got["decision"] != "block" {
		t.Errorf("default stop = %v", got)
	}
	if got := roundTrip(t, stopBody(Dialect{StopBlock: StopFollowup}, "r")); got["followup_message"] != "r" {
		t.Errorf("followup stop = %v", got)
	}
	if stopBody(Dialect{StopBlock: StopNone}, "r") != nil {
		t.Error("a host with no end-of-turn hook must be sent nothing")
	}
	both := roundTrip(t, postToolBody(Dialect{}, "advice"))
	if both["systemMessage"] != "advice" || both["hookSpecificOutput"] == nil {
		t.Errorf("default post-tool reply must carry both fields: %v", both)
	}
	flat := roundTrip(t, postToolBody(Dialect{PostTool: PostToolFlat}, "advice"))
	if len(flat) != 1 || flat["additional_context"] != "advice" {
		t.Errorf("flat post-tool reply = %v", flat)
	}
}

// Every host must have a row, and the capability flags must agree with what its
// contract says its events can do, so the two tables cannot contradict.
func TestEveryHostHasADialectConsistentWithItsContract(t *testing.T) {
	for _, client := range Clients() {
		contract, ok := HostContractFor(client)
		if !ok {
			t.Fatalf("%s has no contract", client)
		}
		if contract.Dialect.ToolUseNodeReminder && contract.Dialect.PromptInjection {
			t.Errorf("%s reminds of the node after a tool call yet can inject at the prompt; the reminder would repeat what the prompt already said", client)
		}
		if contract.Dialect.StopBlock == StopNone && contract.Dialect.StopAdvisory {
			t.Errorf("%s has no end-of-turn hook but is marked to send a stop advisory", client)
		}
	}
}
