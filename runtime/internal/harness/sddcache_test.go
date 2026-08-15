package harness

import (
	"bytes"
	"strings"
	"testing"
)

// The WebFetch cache pair stayed in Python and each host config named it by a
// relative path. Only Claude Code documents the directory that resolves
// against, so on Cursor and Codex the interpreter was handed a path from an
// unknown starting point and the cache silently never ran.

// A tool that is not WebFetch must not pay for the cache at all. Spawning an
// interpreter on every Bash call would put a process launch in the hot path of
// the most frequent tool there is.
func TestSDDCacheIgnoresEveryOtherTool(t *testing.T) {
	var out bytes.Buffer
	blocked := sddCache(Request{
		WorkspaceRoot: t.TempDir(), ToolkitRoot: toolkitRoot, Client: ClientClaude,
	}, payload{ToolName: "Bash"}, "sdd-cache-pre.py", &out)

	if blocked != nil {
		t.Errorf("a Bash call was sent through the WebFetch cache: %v", blocked)
	}
	if out.Len() != 0 {
		t.Errorf("a Bash call produced cache output: %s", out.String())
	}
}

// A missing script is not a broken session. The cache is an optimisation, and a
// hook that fails because an optional accelerator is absent is worse than one
// that quietly does not accelerate.
func TestSDDCacheIsSilentWhenTheScriptIsAbsent(t *testing.T) {
	var out bytes.Buffer
	blocked := sddCache(Request{
		WorkspaceRoot: t.TempDir(), ToolkitRoot: t.TempDir(), Client: ClientClaude,
	}, payload{ToolName: "WebFetch"}, "sdd-cache-pre.py", &out)

	if blocked != nil {
		t.Errorf("an absent script refused a tool call: %v", blocked)
	}
}

// A refusal from the cache is a refusal on the same event as the safety gate,
// so it has to leave through the same door. It did not at first: the block was
// returned past the per-host translation, and Cursor and Codex received exit 0
// and an empty reply while Claude got the cached page.
func TestACacheRefusalIsDeliveredInEachHostsShape(t *testing.T) {
	reason := &BlockError{Reason: "[sdd-cache] Cache hit"}

	for _, host := range []struct {
		client Client
		expect string
	}{
		{ClientCursor, "permission"},
		{ClientCodex, "permissionDecision"},
	} {
		var out bytes.Buffer
		if err := deliverBlock(Request{Client: host.client}, reason, &out); err != nil {
			t.Fatalf("%s: %v", host.client, err)
		}
		if !strings.Contains(out.String(), host.expect) {
			t.Errorf("%s got no refusal it can read: %s", host.client, out.String())
		}
	}

	// Claude decides by exit status, so its refusal is the returned error and
	// main turns it into exit 2 with the reason on stderr.
	var out bytes.Buffer
	if err := deliverBlock(Request{Client: ClientClaude}, reason, &out); err == nil {
		t.Error("Claude got no error to exit 2 with")
	}
}
