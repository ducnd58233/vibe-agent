package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestUnsupportedHandlerIfRejectsFlatIf(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".cursor")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"version":1,"hooks":{"postToolUse":[{"command":"x","if":"Bash(git *)"}]}}`)
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	problems := UnsupportedHandlerIf(root)
	if len(problems) == 0 {
		t.Fatal("expected an unsupported if to be reported")
	}
}

func TestUnsupportedHandlerIfAllowsCleanCodex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"vibe-agent hook post-tool-use"}]}]}}`)
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if problems := UnsupportedHandlerIf(root); len(problems) != 0 {
		t.Fatalf("clean codex hooks reported: %v", problems)
	}
}

func TestToolkitHostsWithoutIfSupportShipNone(t *testing.T) {
	root := testutil.ToolkitRoot(t)
	if problems := UnsupportedHandlerIf(root); len(problems) != 0 {
		t.Fatalf("configs for hosts that ignore if must not ship one: %s", FormatHandlerIfProblems(problems))
	}
}
