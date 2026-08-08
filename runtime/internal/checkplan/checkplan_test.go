package checkplan

import (
	"os"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	path := DefaultPath(root)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	return root
}

const minimal = `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      args: [test, ./...]
    e2e:
      command: npm
      args: [run, e2e]
      timeoutSeconds: 900
`

func TestPlanResolvesTheCommandAndTimeoutForACheck(t *testing.T) {
	plan, err := Load(DefaultPath(write(t, minimal)))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	entry, err := plan.Entry("e2e")
	if err != nil {
		t.Fatalf("resolve e2e: %v", err)
	}
	if entry.Command != "npm" {
		t.Errorf("command = %q, want npm", entry.Command)
	}
	if got := strings.Join(entry.Args, " "); got != "run e2e" {
		t.Errorf("args = %q, want \"run e2e\"", got)
	}
	if entry.Timeout().Minutes() != 15 {
		t.Errorf("timeout = %v, want 15m", entry.Timeout())
	}
}

// The whole point of the plan is that a check nobody declared cannot quietly
// become a pass. An undeclared check must be an error the caller cannot
// mistake for evidence.
func TestPlanRefusesACheckItDoesNotDeclare(t *testing.T) {
	plan, err := Load(DefaultPath(write(t, minimal)))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, err := plan.Entry("lint"); err == nil {
		t.Fatal("an undeclared check resolved; a missing command must never verify")
	} else if !strings.Contains(err.Error(), "lint") {
		t.Errorf("the error does not name the missing check: %v", err)
	}
}

// A missing plan is the state every repo starts in, so it is the case most
// likely to be reached by accident. It must fail closed and name the file the
// human has to write, not resolve to "nothing to verify".
func TestAMissingPlanIsAnErrorNamingThePath(t *testing.T) {
	root := t.TempDir()
	_, err := Load(DefaultPath(root))
	if err == nil {
		t.Fatal("a missing plan loaded; that would let every check resolve to nothing")
	}
	if !strings.Contains(err.Error(), FileName) {
		t.Errorf("the error does not name %s: %v", FileName, err)
	}
}

// A typo in a plan key would otherwise drop the setting it was meant to carry,
// and the check would run with a default nobody chose.
func TestPlanRejectsAnUnknownField(t *testing.T) {
	root := write(t, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      commnad: go
`)
	if _, err := Load(DefaultPath(root)); err == nil {
		t.Fatal("a misspelled key was accepted")
	}
}

func TestPlanRejectsAnEntryWithNoCommand(t *testing.T) {
	root := write(t, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      timeoutSeconds: 60
`)
	if _, err := Load(DefaultPath(root)); err == nil {
		t.Fatal("an entry with nothing to run was accepted")
	}
}

func TestPlanRejectsTheWrongKind(t *testing.T) {
	root := write(t, `apiVersion: vibe-agent/v1
kind: WorkflowGraph
spec:
  checks:
    unit:
      command: go
`)
	if _, err := Load(DefaultPath(root)); err == nil {
		t.Fatal("a graph was accepted as a check plan")
	}
}
