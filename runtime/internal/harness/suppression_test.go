package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// edit asks the gate about one Edit: what the text was, and what it is about to
// become.
func edit(t *testing.T, root, before, after string) *BlockError {
	t.Helper()
	var body payload
	body.ToolName = "Edit"
	body.ToolInput.FilePath = filepath.Join(root, "service.go")
	body.ToolInput.OldString = before
	body.ToolInput.NewString = after
	return suppressionVerdict(Request{WorkspaceRoot: root}, body)
}

// Every shape needs a case. Walking the ids means adding one without a test
// fails rather than shipping a rule nobody exercised.
func TestEveryShapeRefusesSomethingBeingAdded(t *testing.T) {
	cases := []struct {
		id     string
		before string
		after  string
	}{
		{"linter-directive", "value := compute()", "value := compute() //nolint:errcheck"},
		{"linter-directive", "total = a + b", "total = a + b  # noqa: E501"},
		{"linter-directive", "const x = read()", "// eslint-disable-next-line no-eval\nconst x = read()"},
		{"linter-directive", "let y: number = get()", "// @ts-ignore\nlet y: number = get()"},
		{"disabled-rule", "linters:\n  enable:\n    - errcheck", "linters:\n  disable: true"},
		{"skipped-test", "func TestThing(t *testing.T) {\n\tcheck(t)", "func TestThing(t *testing.T) {\n\tt.Skip(\"flaky\")"},
		{"skipped-test", "it('adds', () => {})", "it.skip('adds', () => {})"},
		{"skipped-test", "def test_thing():", "@pytest.mark.skip\ndef test_thing():"},
		{"slop-baseline", "slop audit --fail-on 3 .", "slop audit --fail-on 9 ."},
		{"coverage-floor", "--cov-fail-under 90", "--cov-fail-under 40"},
	}

	shapes := SuppressionShapes()
	if len(shapes) == 0 {
		t.Fatal("the built-in suppression plan is empty")
	}
	covered := map[string]bool{}
	for _, testCase := range cases {
		covered[testCase.id] = true
		blocked := edit(t, t.TempDir(), testCase.before, testCase.after)
		if blocked == nil {
			t.Errorf("%s allowed:\n%s", testCase.id, testCase.after)
			continue
		}
		if !strings.Contains(blocked.Reason, testCase.id) {
			t.Errorf("%s refused without naming itself:\n%s", testCase.id, blocked.Reason)
		}
	}

	for _, id := range shapes {
		if !covered[id] {
			t.Errorf("shape %q has no test case; add one rather than shipping an unexercised rule", id)
		}
	}
}

// The criterion this whole design turns on. Nearly every repository has
// suppressions it decided on deliberately, and a rule that fired on those would
// be switched off within a day.
func TestMovingAnExistingSuppressionIsNotAddingOne(t *testing.T) {
	before := "func a() {\n\tvalue := compute() //nolint:errcheck\n}"
	after := "func b() {\n\tvalue := compute() //nolint:errcheck\n}"

	if blocked := edit(t, t.TempDir(), before, after); blocked != nil {
		t.Errorf("moving a suppression was refused:\n%s", blocked.Reason)
	}
}

// Removing one is the direction the gate wants, and must never be refused.
func TestRemovingASuppressionIsNeverRefused(t *testing.T) {
	before := "value := compute() //nolint:errcheck\nother := compute() //nolint:errcheck"
	after := "value := compute()\nother := compute()"

	if blocked := edit(t, t.TempDir(), before, after); blocked != nil {
		t.Errorf("removing suppressions was refused:\n%s", blocked.Reason)
	}
}

// A refusal that fires on honest work gets the whole gate switched off.
func TestOrdinaryEditsAreLeftAlone(t *testing.T) {
	for _, testCase := range [][2]string{
		{"a := 1", "a := 2"},
		{"// the linter wants this checked", "// the linter wants this checked, and it is"},
		{"func TestThing(t *testing.T) {", "func TestThing(t *testing.T) {\n\tt.Parallel()"},
		{"threshold := 3", "threshold := 9"},
		{"", "package main\n\nfunc main() {}"},
	} {
		if blocked := edit(t, t.TempDir(), testCase[0], testCase[1]); blocked != nil {
			t.Errorf("an ordinary edit was refused:\n%s\n%s", testCase[1], blocked.Reason)
		}
	}
}

// A tightened threshold is the opposite of the finding.
func TestTighteningAThresholdIsNotRefused(t *testing.T) {
	if blocked := edit(t, t.TempDir(), "--fail-on 9", "--fail-on 3"); blocked != nil {
		t.Errorf("lowering the slop ceiling was refused:\n%s", blocked.Reason)
	}
	if blocked := edit(t, t.TempDir(), "--cov-fail-under 40", "--cov-fail-under 90"); blocked != nil {
		t.Errorf("raising the coverage floor was refused:\n%s", blocked.Reason)
	}
}

// A refusal has to say what to do instead, or the next move is to work around it.
func TestARefusalSaysWhatToDoInstead(t *testing.T) {
	blocked := edit(t, t.TempDir(), "value := compute()", "value := compute() //nolint:errcheck")
	if blocked == nil {
		t.Fatal("a nolint was allowed")
	}
	for _, want := range []string{"Fix the code", "stop and report", SuppressionAllowMarker, "nolint"} {
		if !strings.Contains(blocked.Reason, want) {
			t.Errorf("the refusal does not contain %q:\n%s", want, blocked.Reason)
		}
	}
}

// One deliberate suppression, acknowledged on its line, is the escape hatch.
// Greppable so every exemption in a repository can be listed and reviewed.
func TestAnAcknowledgedSuppressionIsAllowed(t *testing.T) {
	after := "value := compute() //nolint:errcheck // " + SuppressionAllowMarker + " the error is the loop exit"
	if blocked := edit(t, t.TempDir(), "value := compute()", after); blocked != nil {
		t.Errorf("an acknowledged suppression was refused:\n%s", blocked.Reason)
	}
}

// Write sends no before, so the file it is replacing is what the gate compares
// against. Without this, rewriting a file with its suppressions intact reads as
// adding every one of them.
func TestAWriteComparesAgainstTheFileOnDisk(t *testing.T) {
	root := t.TempDir()
	existing := "package a\n\nfunc a() {\n\tvalue := compute() //nolint:errcheck\n}\n"
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	var body payload
	body.ToolName = "Write"
	body.ToolInput.FilePath = filepath.Join(root, "service.go")
	body.ToolInput.Content = "package a\n\nfunc b() {\n\tvalue := compute() //nolint:errcheck\n}\n"

	if blocked := suppressionVerdict(Request{WorkspaceRoot: root}, body); blocked != nil {
		t.Errorf("rewriting a file with its own suppression was refused:\n%s", blocked.Reason)
	}

	body.ToolInput.Content = existing + "\nfunc c() {\n\tother := compute() //nolint:errcheck\n}\n"
	if blocked := suppressionVerdict(Request{WorkspaceRoot: root}, body); blocked == nil {
		t.Error("a Write that added a second suppression was allowed")
	}
}

// A new file has nothing to compare against, so every suppression in it is one
// being added. That is the strict direction, and it is the right one.
func TestANewFileFullOfSuppressionsIsRefused(t *testing.T) {
	root := t.TempDir()
	var body payload
	body.ToolName = "Write"
	body.ToolInput.FilePath = filepath.Join(root, "new.go")
	body.ToolInput.Content = "package a\n\nfunc a() {\n\tvalue := compute() //nolint:errcheck\n}\n"

	if blocked := suppressionVerdict(Request{WorkspaceRoot: root}, body); blocked == nil {
		t.Error("a new file carrying a suppression was allowed")
	}
}

// A tool call that writes nothing has nothing to say about.
func TestACallThatWritesNothingIsIgnored(t *testing.T) {
	var body payload
	body.ToolName = "Bash"
	body.ToolInput.Command = "go test ./..."
	if blocked := suppressionVerdict(Request{WorkspaceRoot: t.TempDir()}, body); blocked != nil {
		t.Errorf("a shell command was refused by the suppression gate:\n%s", blocked.Reason)
	}
}

// A consumer extends the list. It cannot shorten it.
func TestAConsumerPlanAddsShapesAndCannotRemoveThem(t *testing.T) {
	root := t.TempDir()
	scoped, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scoped.Close() }()
	if err := scoped.MkdirAll(".ai-agents", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := scoped.WriteFile(filepath.Join(".ai-agents", "suppression.yaml"), []byte(`
apiVersion: vibe-agent/v1
kind: SuppressionPlan
spec:
  shapes:
    - id: house-rule
      reason: This repository says so.
      patterns: ['(?i)//\s*house-override\b']
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if blocked := edit(t, root, "a := 1", "a := 1 // house-override"); blocked == nil {
		t.Error("a consumer shape did not take effect")
	}
	if blocked := edit(t, root, "a := 1", "a := 1 //nolint:errcheck"); blocked == nil {
		t.Error("a consumer plan displaced the built-in shapes")
	}
}

func TestABrokenConsumerPlanLeavesTheBuiltInShapesStanding(t *testing.T) {
	root := t.TempDir()
	scoped, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scoped.Close() }()
	if err := scoped.MkdirAll(".ai-agents", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := scoped.WriteFile(filepath.Join(".ai-agents", "suppression.yaml"),
		[]byte("this: is: not: a: plan\n  - ["), 0o600); err != nil {
		t.Fatal(err)
	}

	if blocked := edit(t, root, "a := 1", "a := 1 //nolint:errcheck"); blocked == nil {
		t.Error("a malformed consumer plan switched the gate off")
	}
}

// Compiling at load is what turns a broken pattern into a named failure rather
// than a rule that silently never fires.
func TestSuppressionPlanValidationNamesWhatIsWrong(t *testing.T) {
	cases := []struct {
		name string
		plan string
		want string
	}{
		{
			name: "uncompilable pattern",
			plan: "spec:\n  shapes:\n    - id: broken\n      reason: Testing.\n      patterns: ['(unclosed']",
			want: "broken",
		},
		{
			name: "no reason",
			plan: "spec:\n  shapes:\n    - id: silent\n      patterns: ['x']",
			want: "silent",
		},
		{
			name: "threshold with no direction",
			plan: "spec:\n  thresholds:\n    - id: nodir\n      reason: Testing.\n      pattern: 'x(\\d+)'",
			want: "nodir",
		},
		{
			name: "threshold capturing nothing",
			plan: "spec:\n  thresholds:\n    - id: nogroup\n      reason: Testing.\n      pattern: 'x'\n      direction: increase",
			want: "nogroup",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := parseSuppressionPlan([]byte("apiVersion: vibe-agent/v1\nkind: SuppressionPlan\n" + testCase.plan))
			if err == nil {
				t.Fatal("a broken plan loaded")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error = %q, want it to name %q", err, testCase.want)
			}
		})
	}
}

// The built-in plan has to compile, or every refusal in it is inert.
func TestTheBuiltInSuppressionPlanCompiles(t *testing.T) {
	plan, err := parseSuppressionPlan(suppressionDefaultPlan)
	if err != nil {
		t.Fatalf("the shipped suppression plan does not load: %v", err)
	}
	if len(plan.shapes)+len(plan.thresholds) != len(SuppressionShapes()) {
		t.Errorf("plan has %d entries, SuppressionShapes reports %d",
			len(plan.shapes)+len(plan.thresholds), len(SuppressionShapes()))
	}
}

// The file that defines a rule has to stay writable, or the rule stops being
// extended. The list is exact, so a consumer cannot widen it by naming a file
// suppression-something.
func TestTheRuleIsWritableInTheFileThatDefinesIt(t *testing.T) {
	root := t.TempDir()
	adds := "value := compute() " + "//nolint" + ":errcheck"

	var body payload
	body.ToolName = "Write"
	body.ToolInput.Content = adds

	body.ToolInput.FilePath = filepath.Join(root, "runtime", "internal", "harness", "suppression_test.go")
	if blocked := suppressionVerdict(Request{WorkspaceRoot: root}, body); blocked != nil {
		t.Errorf("the gate refused its own test file:\n%s", blocked.Reason)
	}

	body.ToolInput.FilePath = filepath.Join(root, "runtime", "internal", "harness", "suppression-helpers.go")
	if blocked := suppressionVerdict(Request{WorkspaceRoot: root}, body); blocked == nil {
		t.Error("a file named suppression-something was exempted; the list is meant to be exact")
	}
}

// The shapes a rule this blunt gets wrong. Each of these was either in the
// pattern list once or is the obvious next thing to add, and each is the
// opposite of the finding: a config that starts enabling rules, a test that
// declares it can run alongside others.
func TestConfigurationThatEnablesRulesIsNotADisabledRule(t *testing.T) {
	for _, after := range []string{
		"rules:",
		"linters:\n  enable:\n    - errcheck\n    - gosec",
		"\"rules\": {",
		"func TestThing(t *testing.T) {\n\tt.Parallel()\n\tcheck(t)",
	} {
		if blocked := edit(t, t.TempDir(), "# config", after); blocked != nil {
			t.Errorf("enabling work was refused:\n%s\n%s", after, blocked.Reason)
		}
	}
}

// Adding a ceiling where there was none is adding a check, not weakening one.
func TestIntroducingAThresholdIsNotWideningOne(t *testing.T) {
	if blocked := edit(t, t.TempDir(), "slop audit .", "slop audit --fail-on 5 ."); blocked != nil {
		t.Errorf("introducing a ceiling was refused:\n%s", blocked.Reason)
	}
}
