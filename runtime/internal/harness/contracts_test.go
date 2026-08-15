package harness

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// updateDoc regenerates the committed reference instead of asserting on it.
//
//	go -C runtime test ./internal/harness -run TestHostContractsDoc -update
var updateDoc = flag.Bool("update", false, "rewrite the generated host contract reference")

// The document and the table are one fact with two renderings, and the test is
// what keeps them one. Without it this becomes the second copy that drifts,
// which is the failure d4c0b0c was written to clean up.
func TestHostContractsDocMatchesTheTable(t *testing.T) {
	path := filepath.Join(toolkitRoot, HostContractsDoc)
	rendered := RenderHostContracts()

	if *updateDoc {
		if err := os.WriteFile(path, []byte(rendered), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("regenerated %s", path)
		return
	}

	committed, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with -update)", path, err)
	}
	if normalise(string(committed)) != normalise(rendered) {
		t.Errorf("%s is out of date with contracts.go; regenerate with:\n"+
			"  go -C runtime test ./internal/harness -run TestHostContractsDoc -update", HostContractsDoc)
	}
}

// normalise removes the line-ending difference between a file checked out on
// Windows and a string built in memory. Failing a test over CRLF would teach
// people to ignore it.
func normalise(text string) string {
	return strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n")
}

// A source URL is what separates a recorded contract from a remembered one.
// Rule 1 of the table, checked rather than trusted.
func TestEveryHostContractCitesASource(t *testing.T) {
	for _, host := range HostContracts() {
		if !strings.HasPrefix(host.Source, "https://") {
			t.Errorf("%s cites %q, which is not a source anyone can open", host.Client, host.Source)
		}
		if host.ConfigPath == "" {
			t.Errorf("%s names no config path", host.Client)
		}
	}
}

// An unverified row that does not say why is indistinguishable from one nobody
// finished writing, and the difference decides whether to trust it.
func TestUnverifiedRowsExplainThemselves(t *testing.T) {
	for _, host := range HostContracts() {
		for _, event := range host.Events {
			if !event.Verified && event.Why == "" {
				t.Errorf("%s %s is unverified and gives no reason", host.Client, event.HostKey)
			}
			if event.Verified && event.Why != "" {
				t.Errorf("%s %s is verified but still carries a reason: %s", host.Client, event.HostKey, event.Why)
			}
		}
	}
}

// Every host this build answers must have a contract, or doctor would check
// some configs against a table and wave the rest through.
func TestEveryAnsweredClientHasAContract(t *testing.T) {
	for _, client := range Clients() {
		if _, ok := HostContractFor(client); !ok {
			t.Errorf("this build answers %s and has no contract for it", client)
		}
	}
}

// Cursor is the host whose wiring was written from a vendor page and never
// watched running. Recording that is the point; a row quietly flipped to
// verified without a measurement would put this table back where the prose was.
func TestCursorIsRecordedAsUnverifiedUntilMeasured(t *testing.T) {
	contract, ok := HostContractFor(ClientCursor)
	if !ok {
		t.Fatal("no Cursor contract")
	}
	for _, event := range contract.Events {
		if event.Verified {
			t.Errorf("Cursor %s claims verification; no Cursor hook has been observed firing. "+
				"If one now has been, this test is the thing to update, deliberately.", event.HostKey)
		}
	}
}

// Answering a host in another host's shape is the silent failure this whole
// table is against, so every key a host is sent must be one it reads.
//
// This began as an assertion that opencode was described and deliberately not
// answered, and it failed the moment the client joined Clients. That is the
// guard working: it forced the envelope to exist before the name did, rather
// than after someone noticed opencode discarding every reply.
//
// It was then rewritten, because the obvious next version was wrong. Comparing
// envelopes for inequality would have failed Codex, which reads Claude's shape
// deliberately and by measurement. "Has a reply of its own" is not "has a reply
// unlike Claude's"; the property that matters is agreement with the contract.
func TestEveryAnsweredClientIsSentKeysItReads(t *testing.T) {
	for _, client := range Clients() {
		contract, ok := HostContractFor(client)
		if !ok {
			t.Errorf("%s is answered and has no contract", client)
			continue
		}
		event, ok := injectingEvent(contract)
		if !ok {
			continue // a host with no injection point sends nothing to check
		}

		// Through Run, not emitContext. sessionStart writes its own payload
		// rather than calling emitContext, so an earlier version of this test
		// checked a function the real session-start path never reaches, and
		// missed opencode being answered in Claude's nested shape.
		out := bytes.NewBufferString("")
		if err := Run(Request{
			Event: EventSessionStart, Client: client,
			WorkspaceRoot: t.TempDir(), ToolkitRoot: toolkitRoot,
			Stdin: strings.NewReader("{}"),
		}, out); err != nil {
			t.Fatalf("session start (%s): %v", client, err)
		}
		if out.Len() == 0 {
			continue
		}

		var decoded map[string]any
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatalf("%s payload is not JSON: %v: %s", client, err, out.String())
		}
		for _, key := range flattenKeys(decoded, "") {
			if !slices.Contains(event.OutputKeys, key) {
				t.Errorf("%s is sent %q, which its contract does not record it reading; recorded: %v",
					client, key, event.OutputKeys)
			}
		}
	}
}

// injectingEvent returns the host's session-start row, which is where context
// injection is checked.
func injectingEvent(contract HostContract) (EventContract, bool) {
	for _, event := range contract.Events {
		if event.Event == EventSessionStart && len(event.OutputKeys) > 0 {
			return event, true
		}
	}
	return EventContract{}, false
}

// flattenKeys renders nested keys as dotted paths, so a contract can record
// hookSpecificOutput.additionalContext as the single fact it is rather than as
// two levels a test has to reassemble.
func flattenKeys(object map[string]any, prefix string) []string {
	var keys []string
	for key, value := range object {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			keys = append(keys, flattenKeys(nested, path)...)
			continue
		}
		keys = append(keys, path)
	}
	return keys
}
