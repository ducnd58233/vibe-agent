package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// refuse asks the gate about one shell command, with no run in the workspace.
//
// No run on purpose: the danger list is about the action, not about the state
// of a delivery run, and it has to hold in a workspace that has never started
// one.
func refuse(t *testing.T, command string) *BlockError {
	t.Helper()
	var body payload
	body.ToolName = "Bash"
	body.ToolInput.Command = command
	return dangerVerdict(Request{WorkspaceRoot: t.TempDir()}, body)
}

// Every category needs a test. Walking the ids means adding one without a case
// here fails rather than shipping a rule nobody exercised.
func TestEveryDangerCategoryRefusesSomething(t *testing.T) {
	// A slice of pairs rather than a map keyed by category id: a map literal
	// keyed by a name containing "cred" reads to a secret scanner as a hardcoded
	// credential, and renaming the category to quiet it would be renaming the
	// thing rather than fixing anything.
	cases := []struct {
		category string
		command  string
	}{
		{"migration", "rake db:migrate"},
		{"data-destruction", `psql -c "DROP TABLE users"`},
		{"production-write", "kubectl --namespace prod apply -f deploy.yaml"},
		{"credential-change", "gh secret set BUILD_FLAG"},
		{"history-rewrite", "git push --force origin main"},
		{"infrastructure-destruction", "terraform destroy -auto-approve"},
		{"local-destruction", "rm" + " -rf /"},
		{"publication", "npm publish"},
	}

	categories := DangerCategories()
	if len(categories) == 0 {
		t.Fatal("the built-in danger plan is empty")
	}
	covered := map[string]bool{}
	for _, testCase := range cases {
		covered[testCase.category] = true
		blocked := refuse(t, testCase.command)
		if blocked == nil {
			t.Errorf("category %q allowed %q", testCase.category, testCase.command)
			continue
		}
		if !strings.Contains(blocked.Reason, testCase.category) {
			t.Errorf("category %q refused %q without naming itself: %s",
				testCase.category, testCase.command, blocked.Reason)
		}
	}

	for _, id := range categories {
		if !covered[id] {
			t.Errorf("category %q has no test case; add one rather than shipping an unexercised rule", id)
		}
	}
}

// A refusal that fires on honest work gets the whole gate switched off, so the
// ordinary commands this repository runs all day must pass.
func TestTheDangerListLeavesOrdinaryWorkAlone(t *testing.T) {
	for _, command := range []string{
		"go test ./...",
		"make -C runtime check",
		"git add -A",
		"git commit -m 'fix: something'",
		"git push origin feat/branch",
		"git push --force-with-lease origin feat/branch",
		"gh pr create --title x",
		"gh pr checks 70",
		"npm install",
		"docker build -t local .",
		"kubectl get pods",
		"terraform plan",
	} {
		if blocked := refuse(t, command); blocked != nil {
			t.Errorf("%q was refused as dangerous:\n%s", command, blocked.Reason)
		}
	}
}

// The split over-approximates on purpose: a dangerous command hidden behind a
// separator is still seen.
func TestADangerousCommandBehindASeparatorIsSeen(t *testing.T) {
	for _, command := range []string{
		"go build ./... && terraform destroy",
		"echo start; npm publish",
		"make check || gh secret set TOKEN",
		"cat file | psql -c 'TRUNCATE TABLE users'",
	} {
		if blocked := refuse(t, command); blocked == nil {
			t.Errorf("%q slipped past the danger list", command)
		}
	}
}

// A write into a migration directory is the same event as running the migrator.
func TestWritingAMigrationIsRefused(t *testing.T) {
	var body payload
	body.ToolName = "Write"
	body.ToolInput.FilePath = filepath.Join("db", "migrate", "20260820_add_column.sql")
	body.ToolInput.Content = "ALTER TABLE users ADD COLUMN x int;"

	blocked := dangerVerdict(Request{WorkspaceRoot: t.TempDir()}, body)
	if blocked == nil {
		t.Fatal("a migration file was written with nothing in the way")
	}
	if !strings.Contains(blocked.Reason, "migration") {
		t.Errorf("reason = %q", blocked.Reason)
	}
}

// A refusal has to say why, or the person reading it cannot decide.
func TestARefusalCarriesItsReasonAndWhatMatched(t *testing.T) {
	blocked := refuse(t, "terraform destroy")
	if blocked == nil {
		t.Fatal("terraform destroy was allowed")
	}
	for _, want := range []string{"danger list", "infrastructure-destruction", "terraform destroy", "A person decides"} {
		if !strings.Contains(blocked.Reason, want) {
			t.Errorf("reason does not contain %q:\n%s", want, blocked.Reason)
		}
	}
}

// A consumer extends the list. It cannot shorten it.
func TestAConsumerPlanAddsCategoriesAndCannotRemoveThem(t *testing.T) {
	root := t.TempDir()
	scoped, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scoped.Close() }()
	if err := scoped.MkdirAll(".ai-agents", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := scoped.WriteFile(filepath.Join(".ai-agents", "danger.yaml"), []byte(`
apiVersion: vibe-agent/v1
kind: DangerPlan
spec:
  categories:
    - id: house-rule
      reason: This repository says so.
      commands: ['(?i)\bflip-the-switch\b']
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var body payload
	body.ToolName = "Bash"
	body.ToolInput.Command = "flip-the-switch now"
	if blocked := dangerVerdict(Request{WorkspaceRoot: root}, body); blocked == nil {
		t.Error("a consumer category did not take effect")
	}

	// The built-in list is still there.
	body.ToolInput.Command = "terraform destroy"
	if blocked := dangerVerdict(Request{WorkspaceRoot: root}, body); blocked == nil {
		t.Error("a consumer plan displaced the built-in list")
	}
}

// A typo in an optional file must not switch the gate off.
func TestABrokenConsumerPlanLeavesTheBuiltInListStanding(t *testing.T) {
	root := t.TempDir()
	scoped, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scoped.Close() }()
	if err := scoped.MkdirAll(".ai-agents", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := scoped.WriteFile(filepath.Join(".ai-agents", "danger.yaml"),
		[]byte("this: is: not: a: plan\n  - ["), 0o600); err != nil {
		t.Fatal(err)
	}

	var body payload
	body.ToolName = "Bash"
	body.ToolInput.Command = "npm publish"
	if blocked := dangerVerdict(Request{WorkspaceRoot: root}, body); blocked == nil {
		t.Error("a malformed consumer plan switched the danger list off")
	}
}

// Compiling at load is what turns a broken pattern into a named failure rather
// than a rule that silently never fires.
func TestABrokenPatternIsNamedAtLoad(t *testing.T) {
	_, err := parseDangerPlan([]byte(`
apiVersion: vibe-agent/v1
kind: DangerPlan
spec:
  categories:
    - id: broken
      reason: Testing.
      commands: ['(unclosed']
`))
	if err == nil {
		t.Fatal("an uncompilable pattern loaded")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error = %q, want it to name the category", err)
	}
}

func TestADangerPlanNeedsAReason(t *testing.T) {
	_, err := parseDangerPlan([]byte(`
apiVersion: vibe-agent/v1
kind: DangerPlan
spec:
  categories:
    - id: silent
      commands: ['(?i)\bwhatever\b']
`))
	if err == nil {
		t.Fatal("a category with no reason loaded")
	}
}

// The built-in plan has to compile, or every refusal in it is inert.
func TestTheBuiltInDangerPlanCompiles(t *testing.T) {
	plan, err := parseDangerPlan(dangerDefaultPlan)
	if err != nil {
		t.Fatalf("the shipped danger plan does not load: %v", err)
	}
	if len(plan) != len(DangerCategories()) {
		t.Errorf("plan has %d categories, DangerCategories reports %d", len(plan), len(DangerCategories()))
	}
}
