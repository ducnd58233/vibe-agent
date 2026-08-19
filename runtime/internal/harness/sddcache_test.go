package harness

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// The WebFetch cache pair stayed in Python and each host config named it by a
// relative path. Only Claude Code documents the directory that resolves
// against, so on Cursor and Codex the interpreter was handed a path from an
// unknown starting point and the cache silently never ran.

// A tool that is not WebFetch must not pay for the cache at all. Spawning an
// interpreter on every Bash call would put a process launch in the hot path of
// the most frequent tool there is.
func TestSDDCacheIgnoresEveryOtherTool(t *testing.T) {
	blocked := sddCache(Request{
		WorkspaceRoot: t.TempDir(), ToolkitRoot: toolkitRoot, Client: ClientClaude,
	}, payload{ToolName: "Bash"}, "sdd-cache-pre.py")

	if blocked != nil {
		t.Errorf("a Bash call was sent through the WebFetch cache: %v", blocked)
	}
}

// A missing script is not a broken session. The cache is an optimisation, and a
// hook that fails because an optional accelerator is absent is worse than one
// that quietly does not accelerate.
func TestSDDCacheIsSilentWhenTheScriptIsAbsent(t *testing.T) {
	blocked := sddCache(Request{
		WorkspaceRoot: t.TempDir(), ToolkitRoot: t.TempDir(), Client: ClientClaude,
	}, payload{ToolName: "WebFetch"}, "sdd-cache-pre.py")

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

// The scripts cannot import the Go constant, so the runtime hands them the
// resolved directory. This is asserted against the script source rather than by
// running them, because both make a network call before touching the cache and
// a test that needs the network is a test that gets skipped.
//
// The bug this guards: both scripts hardcoded .claude/sdd-cache, so a Cursor or
// opencode session wrote its cache into another host's directory. Nothing
// failed, nothing was reported, and the cache looked like it was working.
func TestTheCacheScriptsTakeTheirDirectoryFromTheRuntime(t *testing.T) {
	for _, name := range []string{"sdd-cache-pre.py", "sdd-cache-post.py"} {
		raw, err := os.ReadFile(filepath.Clean(filepath.Join(toolkitRoot, ".ai-agents", "hooks", name)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		source := string(raw)

		if !strings.Contains(source, workspace.EnvSDDCacheDir) {
			t.Errorf("%s does not read %s, so the runtime cannot place its cache",
				name, workspace.EnvSDDCacheDir)
		}
		for _, host := range []string{".claude", ".cursor", ".codex", ".opencode"} {
			if strings.Contains(source, host+"/sdd-cache") || strings.Contains(source, `"`+host+`"`) {
				t.Errorf("%s still names %s for the cache; derived state has one home", name, host)
			}
		}
		if !strings.Contains(source, workspace.StateDirName) {
			t.Errorf("%s has no %s fallback for a standalone run", name, workspace.StateDirName)
		}
	}
}
