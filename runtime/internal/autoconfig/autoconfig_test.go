package autoconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Absence is a no. A workspace that never answered has not opted in, and the
// answer must not be inferred from anything else.
func TestAWorkspaceWithNoFileHasNotOptedIn(t *testing.T) {
	config, present, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("a missing opt-in was an error: %v", err)
	}
	if present {
		t.Error("an empty workspace reported a config")
	}
	if config.MayMerge() {
		t.Error("a workspace with no file may merge")
	}
}

// The template is the safe answer. Writing it must not be the same as giving
// permission, or `auto init` would grant what it is meant to ask for.
func TestTheTemplateOptsOut(t *testing.T) {
	root := t.TempDir()
	if _, err := Write(root); err != nil {
		t.Fatal(err)
	}

	config, present, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("the file was written and not found")
	}
	if config.MayMerge() {
		t.Error("the generated template granted auto-merge")
	}
}

// A person answering yes is the whole mechanism.
// A person answering yes is the whole mechanism. The on-disk round trip is
// covered by the template test above, so this one asks the parser directly.
func TestAnAnsweredFileGrantsTheMerge(t *testing.T) {
	answered := strings.Replace(Template, "merge: false", "merge: true", 1)
	config, err := Parse([]byte(answered))
	if err != nil {
		t.Fatal(err)
	}
	if !config.MayMerge() {
		t.Error("an answered file did not grant the merge")
	}
}

func TestWriteRefusesToOverwriteAnAnswer(t *testing.T) {
	root := t.TempDir()
	if _, err := Write(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root); err == nil {
		t.Fatal("an existing opt-in was overwritten")
	}
}

func TestParseRefusesAnotherKind(t *testing.T) {
	raw := strings.Replace(Template, "kind: AutoConfig", "kind: CheckPlan", 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("a check plan parsed as an auto config")
	}
}

func TestParseRefusesANegativeBudget(t *testing.T) {
	raw := strings.Replace(Template, "tokens: 0", "tokens: -1", 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("a negative budget parsed")
	}
}

// A file this package cannot read must not be treated as an absent one, or a
// typo would silently become an opt-out that looks like a missing file.
func TestAnUnreadableFileIsAnErrorRatherThanAnAbsence(t *testing.T) {
	root := t.TempDir()
	// os.Root rather than a composed path: it is the root-scoped API, and it
	// keeps the write out of a scanner's traversal analysis without anyone
	// having to argue that a temp directory is safe.
	scoped, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scoped.Close() }()
	if err := scoped.MkdirAll(".agent-state", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := scoped.WriteFile(filepath.Join(".agent-state", FileName),
		[]byte("this: is: not: yaml:\n  - ["), 0o600); err != nil {
		t.Fatal(err)
	}

	_, present, err := Load(root)
	if err == nil {
		t.Fatal("a malformed opt-in loaded cleanly")
	}
	if !present {
		t.Error("a malformed file was reported as absent, which reads as a deliberate opt-out")
	}
}

// The template has to say what the answer means, because the file is the only
// place the question is asked.
func TestTheTemplateExplainsWhatYesMeans(t *testing.T) {
	for _, want := range []string{"merge: false", "danger list", "/ship returned GO", "budgets"} {
		if !strings.Contains(Template, want) {
			t.Errorf("the template does not mention %q", want)
		}
	}
}
