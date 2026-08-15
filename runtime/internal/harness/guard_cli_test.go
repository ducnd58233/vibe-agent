package harness

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The scaffold is the first thing a repository sees of this feature. If it did
// not load, everyone's first edit would look like the edit that broke it.
func TestTheScaffoldThisCommandWritesIsAPlanTheLoaderAccepts(t *testing.T) {
	if err := StarterPlanLoads(); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	if _, err := InitGuardPlan(root, false); err != nil {
		t.Fatalf("InitGuardPlan: %v", err)
	}
	if _, err := Guards(root); err != nil {
		t.Fatalf("a freshly written plan did not load: %v", err)
	}
}

// Commented out on purpose: an init that changed behaviour on the way in would
// be a worse default than no init.
func TestTheScaffoldChangesNothingUntilSomeoneEditsIt(t *testing.T) {
	plain, err := Guards(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if _, err := InitGuardPlan(root, false); err != nil {
		t.Fatal(err)
	}
	scaffolded, err := Guards(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(plain) != len(scaffolded) {
		t.Fatalf("init changed the guard set: %d before, %d after", len(plain), len(scaffolded))
	}
	for index := range plain {
		if plain[index].Name != scaffolded[index].Name ||
			len(plain[index].Checks) != len(scaffolded[index].Checks) {
			t.Errorf("init changed %q", plain[index].Name)
		}
	}
}

// The file it would replace holds a repository's own decisions.
func TestInitRefusesToReplaceAPlanSomeoneWrote(t *testing.T) {
	root := t.TempDir()
	path, err := InitGuardPlan(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := InitGuardPlan(root, false); !errors.Is(err, ErrGuardPlanExists) {
		t.Fatalf("init overwrote an existing plan, err = %v", err)
	}
	kept, err := os.ReadFile(filepath.Clean(path))
	if err != nil || string(kept) != "# mine\n" {
		t.Fatalf("the existing plan was not left alone: %q %v", kept, err)
	}

	if _, err := InitGuardPlan(root, true); err != nil {
		t.Fatalf("--force did not replace the plan: %v", err)
	}
}

// Listing is how a repository sees what it is overriding before it overrides
// it, so it has to name every guard and every check.
func TestListingNamesEveryGuardAndCheck(t *testing.T) {
	guards, err := Guards(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(guards) < 4 {
		t.Fatalf("want the four built-in guards, got %d", len(guards))
	}

	seen := map[string]bool{}
	for _, guard := range guards {
		seen[guard.Name] = true
		if guard.Applies == "nothing" {
			t.Errorf("guard %q reads nothing", guard.Name)
		}
		if len(guard.Checks) == 0 {
			t.Errorf("guard %q lists no checks", guard.Name)
		}
	}
	for _, want := range []string{guardSensitiveData, guardCoreLogicTest, guardRawColor, guardUISlop} {
		if !seen[want] {
			t.Errorf("guard %q was not listed", want)
		}
	}
}

// At the command line, unlike in a hook, a person is waiting for an answer. A
// plan that does not load is reported rather than skipped.
func TestListingReportsAPlanThatDoesNotLoad(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(ConsumerGuardPlan))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("kind: NotAGuardPlan\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Guards(root)
	if err == nil || !strings.Contains(err.Error(), ConsumerGuardPlan) {
		t.Fatalf("a broken plan was not reported by name: %v", err)
	}
}
