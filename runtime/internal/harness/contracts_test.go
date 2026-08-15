package harness

import (
	"flag"
	"os"
	"path/filepath"
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

// opencode is named so its gaps are recorded, and withheld from Clients until
// an envelope exists. Answering a host in another host's shape is the silent
// failure this whole table is against.
func TestOpencodeIsDescribedButNotYetAnswered(t *testing.T) {
	if _, ok := HostContractFor(ClientOpencode); !ok {
		t.Fatal("opencode has no contract, so its gaps are recorded nowhere")
	}
	if KnownClient(ClientOpencode) {
		t.Error("opencode is in Clients but has no envelope; it would be answered in Claude's shape and discard every reply")
	}
}
