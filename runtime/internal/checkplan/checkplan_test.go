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
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
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

// Found by mutation testing, not by reading the code. The platform check
// survived having its condition negated, which meant nothing here exercised it:
// the rule was asserted only in the JSON Schema, so the Go loader and the schema
// were one edit away from disagreeing.
func TestPlanRejectsAScreenBlockWithNoPlatform(t *testing.T) {
	root := write(t, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    e2e:
      verifier: screen
      screen:
        expectText: ["Total"]
`)
	_, err := Load(DefaultPath(root))
	if err == nil {
		t.Fatal("a screen block with no platform was accepted; it cannot pick a toolchain")
	}
	if !strings.Contains(err.Error(), "platform") {
		t.Errorf("the error does not say what is missing: %v", err)
	}
}

func TestPlanAcceptsAScreenBlockWithAPlatform(t *testing.T) {
	root := write(t, `apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    e2e:
      verifier: screen
      screen:
        platform: android
        expectText: ["Total"]
`)
	plan, err := Load(DefaultPath(root))
	if err != nil {
		t.Fatalf("a valid screen block was refused: %v", err)
	}
	entry, err := plan.Entry("e2e")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if entry.Screen == nil || entry.Screen.Platform != "android" {
		t.Errorf("the screen block did not survive loading: %+v", entry.Screen)
	}
}

// The schema bounds a timeout at 1 second and this loader did not, so the two
// statements of the same contract could drift.
func TestPlanRejectsANegativeBound(t *testing.T) {
	for _, body := range []string{
		`apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    unit:
      command: go
      timeoutSeconds: -30
`,
		`apiVersion: vibe-agent/v1
kind: CheckPlan
spec:
  checks:
    e2e:
      verifier: screen
      screen:
        platform: android
        settleSeconds: -5
`,
	} {
		if _, err := Load(DefaultPath(write(t, body))); err == nil {
			t.Errorf("a negative bound was accepted:\n%s", body)
		}
	}
}

// An entry with no timeout must fall back to the package default, not to zero.
// Zero reaches context.WithTimeout as an already-expired deadline, so every
// check would time out the moment it started. Nothing asserted the default,
// which is why the boundary survived mutation.
func TestAnEntryWithNoTimeoutGetsTheDefault(t *testing.T) {
	plan, err := Load(DefaultPath(write(t, minimal)))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	entry, err := plan.Entry("unit")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := entry.Timeout(); got != DefaultTimeout {
		t.Errorf("Timeout() = %v, want %v; zero would expire before the check ran", got, DefaultTimeout)
	}
}
