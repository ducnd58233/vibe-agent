package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adviseOn writes a file into a fresh workspace and returns what the guards say
// about it, which is the only surface a host ever sees.
func adviseOn(t *testing.T, name, body string) string {
	t.Helper()
	root := t.TempDir()
	return adviseIn(t, root, name, body)
}

func adviseIn(t *testing.T, root, name, body string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	var body2 payload
	body2.ToolName = "Write"
	body2.ToolInput.FilePath = full
	return adviseAll(Request{WorkspaceRoot: root}, body2)
}

// The point of moving targeting to enry: a stack nobody wrote down is guarded
// anyway. None of these languages is named in guards-default.yaml, and the old
// extension lists covered none of them.
func TestAStackNobodyListedIsStillGuarded(t *testing.T) {
	for _, probe := range []struct{ name, body string }{
		{"main.tf", "locals {\n  api_token = \"9f3a1c2b4d5e6f7a\"\n}\n"},
		{"lib/auth.rb", "API_TOKEN = \"9f3a1c2b4d5e6f7a\"\n"},
		{"src/Auth.php", "<?php\n$api_token = \"9f3a1c2b4d5e6f7a\";\n"},
		{"lib/auth.dart", "const apiToken = \"9f3a1c2b4d5e6f7a\";\n"},
		{"Auth.kt", "val apiToken = \"9f3a1c2b4d5e6f7a\"\n"},
	} {
		said := adviseOn(t, probe.name, probe.body)
		if !strings.Contains(said, "hardcoded-credential") {
			t.Errorf("%s was not guarded: %q", probe.name, said)
		}
	}
}

// Prose is a language type of its own, which is how a design document
// discussing a password stops being a finding. This was 17% of the noise in the
// audit that prompted the port.
func TestADocumentDiscussingSecretsIsNotAFinding(t *testing.T) {
	said := adviseOn(t, "docs/auth.md",
		"# Auth\n\nThe api_token is read from the secret store, never committed.\n"+
			"Set `password = \"something-long\"` only in the local example file.\n")
	if said != "" {
		t.Errorf("prose was scanned as code: %q", said)
	}
}

func TestVendoredCodeIsNotTheAuthorsProblem(t *testing.T) {
	if said := adviseOn(t, "vendor/lib/auth.go",
		"package lib\n\nconst apiToken = \"9f3a1c2b4d5e6f7a\"\n"); said != "" {
		t.Errorf("vendored file was guarded: %q", said)
	}
}

// The credential rule is the noisiest, so a value that reads as an example has
// to stay quiet or the guard trains people to ignore it.
func TestAnExampleCredentialIsNotReported(t *testing.T) {
	for _, body := range []string{
		"const apiToken = \"your-token-here\";\n",
		"const apiToken = process.env.API_TOKEN;\n",
		"const apiToken = \"changeme-please\";\n",
		"const apiToken = \"<your-token>\";\n",
	} {
		if said := adviseOn(t, "cfg.ts", body); strings.Contains(said, "hardcoded-credential") {
			t.Errorf("%q was reported: %s", body, said)
		}
	}
}

func TestAcknowledgingALineSilencesThatLineOnly(t *testing.T) {
	said := adviseOn(t, "cfg.ts",
		"const apiToken = \"9f3a1c2b4d5e6f7a\"; // sensitive-data-guard: allow - fixture\n"+
			"const sessionId = \"8b2c4d6e8f0a1b3c\";\n")

	if strings.Contains(said, "L1 ") {
		t.Errorf("the acknowledged line was still reported: %s", said)
	}
	if !strings.Contains(said, "L2 ") {
		t.Errorf("the unacknowledged line was not reported: %s", said)
	}
}

// The raw-colour guard's own message has always told people to put the marker
// near the file header, so its escape hatch covers the file.
func TestTheColourGuardsMarkerCoversTheWholeFile(t *testing.T) {
	body := "/* design-token-guard: allow-raw-color - brand palette */\n" +
		".a { color: #ff0000; }\n.b { color: #00ff00; }\n"
	if said := adviseOn(t, "brand.css", body); strings.Contains(said, "design-token-guard") {
		t.Errorf("a file-level acknowledgement was ignored: %s", said)
	}
}

// RE2 has no lookaround, so the boundary is checked against the text. A hex run
// inside a longer identifier is not a colour, and two colours separated by one
// character are both colours.
func TestAColourIsBoundedWithoutLookaround(t *testing.T) {
	if said := adviseOn(t, "ids.css", "#nav-1a2b3c { color: inherit; }\n"); said != "" {
		t.Errorf("an identifier was read as a colour: %s", said)
	}
	// Findings are per line, so this is two. The case that matters is that the
	// second literal is found at all: a lookaround folded into the pattern would
	// have consumed the space after `#fff` and left `#000` unanchored.
	said := adviseOn(t, "two.css", ".a { color: #fff; }\n.b { background: #000; }\n")
	if !strings.Contains(said, "2 raw colour value(s)") {
		t.Errorf("adjacent colours were not both found: %s", said)
	}
	if same := adviseOn(t, "one.css", ".a { color: #fff; background: #000; }\n"); !strings.Contains(same, "raw-color") {
		t.Errorf("two colours on one line found none: %s", same)
	}
}

// A test for a parser states its input as source code. Without masking, the
// quoted declaration ends the block early and the quoted assertion counts as a
// real one, so the guard is wrong in both directions on exactly the files that
// most need it.
func TestAFixtureQuotingSourceDoesNotFoolTheTestGuard(t *testing.T) {
	body := "package x\n\n" +
		"import \"testing\"\n\n" +
		"func TestParserHandlesADeclaration(t *testing.T) {\n" +
		"\tinput := `\nfunc Helper() {}\nassert something\n`\n" +
		"\tif Parse(input) == nil {\n\t\tt.Fatal(\"want a tree\")\n\t}\n" +
		"}\n"

	if said := adviseOn(t, "parser_test.go", body); said != "" {
		t.Errorf("a real test was flagged because its fixture quoted code: %s", said)
	}
}

func TestATestThatAssertsNothingIsReported(t *testing.T) {
	body := "package x\n\nimport \"testing\"\n\n" +
		"func TestItRuns(t *testing.T) {\n\tDoWork()\n}\n"
	said := adviseOn(t, "work_test.go", body)
	if !strings.Contains(said, "asserts nothing") {
		t.Errorf("a test with no assertion was not reported: %s", said)
	}
}

func TestASkippedTestIsNotReported(t *testing.T) {
	body := "package x\n\nimport \"testing\"\n\n" +
		"func TestItRuns(t *testing.T) {\n\tt.Skip(\"pending\")\n\tDoWork()\n}\n"
	if said := adviseOn(t, "work_test.go", body); said != "" {
		t.Errorf("a deliberately skipped test was reported: %s", said)
	}
}

// A guard is only useful in a monorepo if a team can add its own rule without
// forking the toolkit.
func TestAConsumerPlanAddsARuleForItsOwnStack(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, `
apiVersion: vibe-agent/v1
kind: GuardPlan
spec:
  guards:
    sensitive-data-guard:
      lineChecks:
        - id: rails-secret-in-log
          pattern: 'Rails\.logger.*{{secret}}'
          message: Rails logger receives authentication material.
`)

	said := adviseIn(t, root, "app/jobs/sync.rb", "Rails.logger.info(user_password)\n")
	if !strings.Contains(said, "rails-secret-in-log") {
		t.Errorf("a consumer rule did not run: %s", said)
	}
}

func TestAConsumerPlanCanDisableOneCheckWithoutLosingTheRest(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, `
apiVersion: vibe-agent/v1
kind: GuardPlan
spec:
  guards:
    sensitive-data-guard:
      disabledChecks: [hardcoded-credential]
`)

	said := adviseIn(t, root, "cfg.ts",
		"const apiToken = \"9f3a1c2b4d5e6f7a\";\nconsole.log(user_password);\n")
	if strings.Contains(said, "hardcoded-credential") {
		t.Errorf("a disabled check still ran: %s", said)
	}
	if !strings.Contains(said, "credential-in-log") {
		t.Errorf("disabling one check silenced the others: %s", said)
	}
}

func TestAConsumerPlanCanDisableAWholeGuard(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, `
apiVersion: vibe-agent/v1
kind: GuardPlan
spec:
  guards:
    design-token-guard:
      disabled: true
`)

	if said := adviseIn(t, root, "brand.css", ".a { color: #ff0000; }\n"); said != "" {
		t.Errorf("a disabled guard still ran: %s", said)
	}
}

// A typo in a tracked file must not quietly remove the protection it was
// extending. It says so once and keeps the built-in rules.
func TestABrokenPlanFallsBackToTheBuiltInRulesAndSaysSo(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, `
apiVersion: vibe-agent/v1
kind: GuardPlan
spec:
  guards:
    sensitive-data-guard:
      lineChecks:
        - id: broken
          pattern: '(?<!x)unsupported'
          message: nope
`)

	said := adviseIn(t, root, "cfg.ts", "const apiToken = \"9f3a1c2b4d5e6f7a\";\n")
	if !strings.Contains(said, "hardcoded-credential") {
		t.Errorf("a broken plan silenced the built-in rules: %s", said)
	}
	if !strings.Contains(said, "[guards]") {
		t.Errorf("a broken plan was not reported: %s", said)
	}
}

// A guard that matches no file looks exactly like a guard with nothing to say,
// which is the failure this port exists to end. It is refused at load.
func TestAGuardThatSelectsNoFileIsRefused(t *testing.T) {
	_, err := buildRules(&guardPlan{
		APIVersion: guardPlanVersion,
		Kind:       guardPlanKind,
		Spec: planSpec{Guards: map[string]guardConfig{
			"empty-guard": {Line: []patternRule{{ID: "x", Pattern: "y", Message: "z"}}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "selects no files") {
		t.Errorf("a guard with no selector was accepted: %v", err)
	}
}

func TestAnUnknownVocabularyReferenceNamesTheFragment(t *testing.T) {
	_, err := buildRules(&guardPlan{
		APIVersion: guardPlanVersion,
		Kind:       guardPlanKind,
		Spec: planSpec{Guards: map[string]guardConfig{
			"sensitive-data-guard": {Line: []patternRule{
				{ID: "typo", Pattern: "{{secrets}}", Message: "m"},
			}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "secrets") {
		t.Errorf("an unknown fragment was not named: %v", err)
	}
}

// The built-in plan is data, so a typo in it would be a runtime failure rather
// than a compile error. This is the test that keeps it a build failure.
func TestTheBuiltInPlanCompiles(t *testing.T) {
	sets, err := buildRules(nil)
	if err != nil {
		t.Fatalf("built-in guard plan does not compile: %v", err)
	}
	if len(sets) < 4 {
		t.Fatalf("want the four built-in guards, got %d", len(sets))
	}
	for _, set := range sets {
		if set.Subject == "" {
			t.Errorf("guard %q has no subject noun", set.Name)
		}
	}
}

// A Read carries a file_path too. The host configs used to filter by matcher,
// and that filtering does not survive one binary answering every event.
func TestReadingAFileIsNotWritingIt(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, "cfg.ts")
	if err := os.WriteFile(full, []byte("const apiToken = \"9f3a1c2b4d5e6f7a\";\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var body payload
	body.ToolName = "Read"
	body.ToolInput.FilePath = full
	if said := adviseAll(Request{WorkspaceRoot: root}, body); said != "" {
		t.Errorf("a Read was scanned as a write: %s", said)
	}
}

func writePlan(t *testing.T, root, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(ConsumerGuardPlan))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
