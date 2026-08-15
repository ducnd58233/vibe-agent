package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// These two checks are worth exactly as much as their failing cases. A check
// that only ever passes is indistinguishable from one that is not wired, which
// is the shape of the problem the whole host-half pass was added to fix.

func TestAnEventKeyTheHostDoesNotPublishFails(t *testing.T) {
	root := t.TempDir()
	// PostToolUseFailure is Claude's spelling and Cursor's is postToolUseFailure.
	// Filing one under the other is the realistic mistake: it is a real event
	// name, on the wrong host, and nothing before this check looked at the key.
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "PostToolUseFailure": [{"command": "vibe-agent hook post-tool-use-failure --client cursor"}]
  }
}`)

	var report diagnostics
	checkHostEventKeys(&report, root)

	if report.problems == 0 {
		t.Error("a key the host never fires was accepted; the hook would be registered and dead")
	}
}

func TestAKeyTheHostDoesPublishPasses(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "postToolUseFailure": [{"command": "vibe-agent hook post-tool-use-failure --client cursor"}],
    "beforeShellExecution": [{"command": "vibe-agent hook pre-tool-use --client cursor"}]
  }
}`)

	var report diagnostics
	checkHostEventKeys(&report, root)

	if report.problems != 0 {
		t.Errorf("valid Cursor keys were rejected (%d problems)", report.problems)
	}
}

// A workspace that wires no host at all is a supported configuration, not a
// defect. Reporting one would fail every consumer repository that uses a subset.
func TestNoConfigsMeansNoVerdict(t *testing.T) {
	var report diagnostics
	checkHostEventKeys(&report, t.TempDir())
	checkHookPathsResolve(&report, t.TempDir())

	if report.problems != 0 {
		t.Errorf("an unwired workspace was reported as broken (%d problems)", report.problems)
	}
}

func TestARelativeScriptPathFails(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "postToolUse": [{"command": "python3 .ai-agents/hooks/sdd-cache-post.py"}]
  }
}`)

	var report diagnostics
	checkHookPathsResolve(&report, root)

	if report.problems == 0 {
		t.Error("a path resolved against an undocumented working directory was accepted")
	}
}

// The forms that say where they start from. Failing any of these would push
// people towards hardcoded absolute paths, which break on the next machine.
func TestAResolvedPathPasses(t *testing.T) {
	for name, command := range map[string]string{
		"host variable":      "python3 ${CLAUDE_PROJECT_DIR}/.ai-agents/hooks/sdd-cache-pre.py",
		"posix absolute":     "python3 /opt/vibe/hooks/cache.py",
		"windows absolute":   `python3 C:\vibe\hooks\cache.py`,
		"home relative":      "python3 ~/.vibe-agent/hooks/cache.py",
		"shell substitution": `python3 "$(git rev-parse --show-toplevel)/hooks/cache.py"`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, filepath.Join(".cursor", "hooks.json"),
				`{"hooks": {"postToolUse": [{"command": "`+escapeJSON(command)+`"}]}}`)

			var report diagnostics
			checkHookPathsResolve(&report, root)

			if report.problems != 0 {
				t.Errorf("%s was rejected: %s", name, command)
			}
		})
	}
}

// vibe-agent hook is exempt because it discovers its own workspace. Requiring
// --workspace from it would demand a value three of the four hosts cannot
// supply, and the only way to satisfy that is a hardcoded path.
func TestVibeAgentHookNeedsNoExplicitWorkspace(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "postToolUse": [{"command": "vibe-agent hook post-tool-use --client cursor"}]
  }
}`)

	var report diagnostics
	checkHookPathsResolve(&report, root)

	if report.problems != 0 {
		t.Error("vibe-agent hook was asked for a workspace it resolves itself")
	}
}

// Claude nests the command one array deeper than Cursor. Reading one shape
// would silently find nothing in the other, which is how a check passes by
// examining an empty list.
func TestBothConfigShapesAreRead(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "statusLine": {"type": "command", "command": "echo hooks/not-a-hook.sh"},
  "hooks": {
    "PostToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "python3 rel/claude.py"}]}]
  }
}`)
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {"postToolUse": [{"command": "python3 rel/cursor.py"}]}
}`)

	offenders, checked := cwdDependentPaths(root)
	if checked != 2 {
		t.Fatalf("read %d configs, want both", checked)
	}
	if len(offenders) != 2 {
		t.Fatalf("want one offender per config, got %d: %v", len(offenders), offenders)
	}
	// One from each shape, so a regression that stops reading either is caught
	// by which one went missing rather than by a count.
	joined := strings.Join(offenders, " ")
	for _, want := range []string{"rel/claude.py", "rel/cursor.py"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%s was not found; offenders were %v", want, offenders)
		}
	}
}

// statusLine is not a hook, and neither is a permission pattern. Walking the
// whole file rather than the hooks object would report both.
func TestOnlyTheHooksObjectIsRead(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".claude", "settings.json"), `{
  "statusLine": {"type": "command", "command": "bash scripts/status.sh"},
  "permissions": {"allow": ["Bash(./scripts/link-ai-agents.sh*)"]},
  "hooks": {
    "PostToolUse": [{"hooks": [{"command": "vibe-agent hook post-tool-use --workspace ${CLAUDE_PROJECT_DIR}"}]}]
  }
}`)

	var report diagnostics
	checkHookPathsResolve(&report, root)

	if report.problems != 0 {
		t.Errorf("something outside the hooks object was reported (%d problems)", report.problems)
	}
}

// escapeJSON makes a command safe to embed in the fixtures above.
func escapeJSON(text string) string {
	var out []rune
	for _, char := range text {
		if char == '\\' || char == '"' {
			out = append(out, '\\')
		}
		out = append(out, char)
	}
	return string(out)
}
