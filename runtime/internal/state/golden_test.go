package state

import (
	"os"
	"path/filepath"
	"testing"
)

// goldenPath holds a manifest that scripts/check-schemas.py validates against
// schemas/run-state.schema.json. It lives in this package's own testdata
// directory, which is the Go convention and keeps the fixture next to the code
// that produces it. It is the contract test between the Go writer
// and the JSON Schema: if Save starts emitting a shape the schema rejects, one
// of these two catches it.
//
// Regenerate with: UPDATE_GOLDEN=1 go test ./internal/state -run TestFreshRunMatchesGolden
const goldenPath = "testdata/fresh-run.json"

var update = os.Getenv("UPDATE_GOLDEN") != ""

func TestFreshRunMatchesGolden(t *testing.T) {
	run, err := NewRun("loop-graph-runtime", "add a control plane", "goal-delivery", 50, fixedTime())
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}

	dir := t.TempDir()
	path := ManifestPath(dir, run.Slug)
	if err := Save(path, run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	produced, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(goldenPath, produced, 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", goldenPath)
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (set UPDATE_GOLDEN=1 to create it): %v", err)
	}
	if string(produced) != string(golden) {
		t.Errorf("Save output drifted from the golden manifest.\n got:\n%s\nwant:\n%s", produced, golden)
	}
}

// A fresh run has not entered its graph yet, so currentNode is empty. The
// schema allows that specific case; this test pins the behavior that produces
// it, so nobody "fixes" it by inventing a placeholder node name.
func TestFreshRunHasNoCurrentNode(t *testing.T) {
	run := newTestRun(t)
	if run.CurrentNode != "" {
		t.Errorf("CurrentNode = %q, want empty until the first transition", run.CurrentNode)
	}
}
