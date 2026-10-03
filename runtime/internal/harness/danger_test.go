package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
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

// refuseTool asks the gate about one tool call that carries no shell command,
// in a workspace that either has a running auto run or has none.
func refuseTool(t *testing.T, tool string, autoRun bool) *BlockError {
	t.Helper()
	root := t.TempDir()
	if autoRun {
		root = workspaceWithRun(t, func(run *state.Run) {
			if err := run.SetFlagAt("auto", true, at()); err != nil {
				t.Fatal(err)
			}
		})
	}
	var body payload
	body.ToolName = tool
	return dangerVerdict(Request{WorkspaceRoot: root}, body)
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
	// outward-action reads a tool name, not a command, so it has its own case
	// list below; it is added to covered here so the walk still sees it.
	outwardTool := "mcp__slack__slack_send_message"

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

	covered["outward-action"] = refuseTool(t, outwardTool, true) != nil

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

// Outward actions are refused only where nobody can answer a prompt. The same
// call in an interactive session is left to the host's own permission flow.
func TestOutwardActionsAreRefusedOnAnAutoRunOnly(t *testing.T) {
	for _, tool := range []string{
		"mcp__slack__slack_send_message",
		"mcp__gmail__send_email",
		"mcp__stripe__create_refund",
		"mcp__bank__transfer_funds",
		"mcp__payments__create_payment",
		"mcp__Google_Drive__share_file",
		"mcp__calendar__create_event",
		"mcp__x__tweet",
	} {
		blocked := refuseTool(t, tool, true)
		if blocked == nil {
			t.Errorf("%q was allowed on an auto run", tool)
			continue
		}
		if !strings.Contains(blocked.Reason, "outward-action") || !strings.Contains(blocked.Reason, tool) {
			t.Errorf("%q refusal does not name the category and the tool:\n%s", tool, blocked.Reason)
		}
		if interactive := refuseTool(t, tool, false); interactive != nil {
			t.Errorf("%q was refused with no auto run, which would break interactive work:\n%s", tool, interactive.Reason)
		}
	}
}

// Reads that share a verb's spelling, and the toolkit's own tools, stay allowed
// on an auto run. A pattern that fires on a read gets the gate switched off.
func TestOutwardActionPatternsLeaveReadsAlone(t *testing.T) {
	for _, tool := range []string{
		"mcp__slack__slack_list_channels",
		"mcp__slack__slack_get_thread_replies",
		"mcp__gmail__search_messages",
		"mcp__gmail__read_message",
		"mcp__stripe__list_payments",
		"mcp__stripe__get_invoice",
		"mcp__calendar__list_events",
		"mcp__github__get_pull_request",
		"mcp__github__create_pull_request",
		"mcp__vibe-agent__vibe_checkpoint",
		"mcp__vibe-agent__vibe_verify",
		"Read",
		"Bash",
	} {
		if blocked := refuseTool(t, tool, true); blocked != nil {
			t.Errorf("%q was refused on an auto run:\n%s", tool, blocked.Reason)
		}
	}
}

// A consumer may add tool patterns, and they honour autoOnly the same way.
func TestAConsumerToolPatternCanBeAutoOnlyOrAlways(t *testing.T) {
	plan, err := parseDangerPlan([]byte(`
apiVersion: vibe-agent/v1
kind: DangerPlan
spec:
  categories:
    - id: house-tool
      reason: This repository stops this tool.
      tools: ['^mcp__crm__delete_']
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || len(plan[0].tools) != 1 || plan[0].AutoOnly {
		t.Fatalf("plan parsed wrong: %+v", plan)
	}
}

// A category with only tool patterns still counts as matching something.
func TestADangerCategoryWithOnlyToolsIsValid(t *testing.T) {
	if _, err := parseDangerPlan([]byte(`
apiVersion: vibe-agent/v1
kind: DangerPlan
spec:
  categories:
    - id: tools-only
      reason: A reason.
      tools: ['^mcp__x__']
`)); err != nil {
		t.Errorf("a tools-only category was rejected: %v", err)
	}
}

// The same refusal, reached the way a host reaches it: a Claude PreToolUse
// payload for an MCP call, through the hook entry point.
func TestPreToolUseRefusesAnMCPSendOnAnAutoRunAndAllowsItOtherwise(t *testing.T) {
	const payloadJSON = `{"tool_name":"mcp__slack__slack_send_message","tool_input":{"channel":"C1","text":"hi"}}`

	auto := workspaceWithRun(t, func(run *state.Run) {
		if err := run.SetFlagAt("auto", true, at()); err != nil {
			t.Fatal(err)
		}
	})
	err := runHook(t, Request{
		Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: auto,
		Stdin: strings.NewReader(payloadJSON),
	})
	var blocked *BlockError
	if !asBlock(err, &blocked) {
		t.Fatalf("an MCP send was allowed on an auto run: %v", err)
	}
	if !strings.Contains(blocked.Reason, "outward-action") {
		t.Errorf("the refusal does not name its category: %s", blocked.Reason)
	}

	// A manual run, and a workspace with no run at all, are both interactive.
	for name, root := range map[string]string{"manual run": workspaceWithRun(t), "no run": t.TempDir()} {
		if err := runHook(t, Request{
			Event: EventPreToolUse, Client: ClientClaude, WorkspaceRoot: root,
			Stdin: strings.NewReader(payloadJSON),
		}); err != nil {
			t.Errorf("%s: an MCP send was refused: %v", name, err)
		}
	}
}

// A person's approval at approve_delivery is what the gate waits for. On an auto
// run at its deliver node with delivery_approved recorded as a human_event, the
// outward call is allowed. Every other state still refuses.
func TestAnApprovedDeliveryMayRunOnAnAutoRun(t *testing.T) {
	const tool = "mcp__slack__slack_send_message"
	cases := []struct {
		name    string
		node    string
		source  state.CheckSource
		passed  bool
		blocked bool
	}{
		{"approved by a person at deliver", "deliver", state.SourceHumanEvent, true, false},
		{"approved by a person, but not at deliver", "execute", state.SourceHumanEvent, true, true},
		{"not approved", "deliver", state.SourceHumanEvent, false, true},
		{"approval from a command, not a person", "deliver", state.SourceExitCode, true, true},
	}
	for _, tc := range cases {
		root := workspaceWithRun(t, func(run *state.Run) {
			if err := run.SetFlagAt("auto", true, at()); err != nil {
				t.Fatal(err)
			}
			run.CurrentNode = tc.node
			if err := run.SetCheckAt("delivery_approved", state.Check{Passed: tc.passed, Source: tc.source, At: at()}, at()); err != nil {
				t.Fatal(err)
			}
		})
		var body payload
		body.ToolName = tool
		blocked := dangerVerdict(Request{WorkspaceRoot: root}, body) != nil
		if blocked != tc.blocked {
			t.Errorf("%s: blocked = %v, want %v", tc.name, blocked, tc.blocked)
		}
	}
}
