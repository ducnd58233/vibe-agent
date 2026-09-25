package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The link script once wrote the maintainer's checkout path into every
// generated hook command, and three of those configs were committed. They ran
// on that one machine and pointed at nothing everywhere else.

func TestAHookCommandCarryingAMachinePathFails(t *testing.T) {
	for _, value := range []string{`\"/opt/checkout/repo\"`, `\"D:/work/repo\"`, `C:\\work\\repo`, `\"/srv/dev/repo\"`, `~/repo`} {
		root := t.TempDir()
		writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "stop": [{"command": "vibe-agent hook stop --workspace `+value+` --client cursor"}]
  }
}`)
		offenders, checked := machinePathHookCommands(root)
		if checked != 1 || len(offenders) != 1 {
			t.Errorf("--workspace %s: offenders=%v checked=%d, want one offender", value, offenders, checked)
			continue
		}
		if !strings.Contains(offenders[0], ".cursor") {
			t.Errorf("offender does not name its config: %q", offenders[0])
		}
	}
}

func TestAHookCommandWithoutAMachinePathPasses(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "stop": [{"command": "vibe-agent hook stop --client cursor"}],
    "sessionStart": [{"command": "vibe-agent hook session-start --workspace ${CLAUDE_PROJECT_DIR} --client cursor"}],
    "postToolUse": [{"command": "python3 \"/opt/toolkit/.ai-agents/hooks/sdd-cache-post.py\""}]
  }
}`)
	offenders, checked := machinePathHookCommands(root)
	if checked != 1 || len(offenders) != 0 {
		t.Errorf("offenders=%v checked=%d; walk-up, a host variable, and a script path are all fine", offenders, checked)
	}

	var report diagnostics
	checkHookMachinePaths(&report, root)
	if report.problems != 0 {
		t.Errorf("a clean config was reported (%d problems)", report.problems)
	}
}

// A toolkit cloned outside the workspace is a supported install that discovery
// cannot reach, so its path is legitimate. The same flag pointing inside the
// workspace is redundant and machine-bound.
func TestAToolkitPathIsAcceptedOnlyOutsideTheWorkspace(t *testing.T) {
	outside := filepath.ToSlash(t.TempDir())
	root := t.TempDir()
	inside := filepath.ToSlash(filepath.Join(root, ".vibe-agent"))
	writeConfig(t, root, filepath.Join(".cursor", "hooks.json"), `{
  "hooks": {
    "stop": [{"command": "vibe-agent hook stop --toolkit \"`+outside+`\" --client cursor"}],
    "sessionStart": [{"command": "vibe-agent hook session-start --toolkit \"`+inside+`\" --client cursor"}]
  }
}`)
	offenders, _ := machinePathHookCommands(root)
	if len(offenders) != 1 || !strings.Contains(offenders[0], ".vibe-agent") {
		t.Errorf("offenders = %v, want only the toolkit inside the workspace", offenders)
	}
}

func TestThisRepositorysHookConfigsCarryNoMachinePath(t *testing.T) {
	offenders, checked := machinePathHookCommands(filepath.Join("..", ".."))
	if checked == 0 {
		t.Fatal("no host configs found in this repository; the check read nothing")
	}
	if len(offenders) != 0 {
		t.Errorf("committed hook configs carry a machine path: %v", offenders)
	}
}
