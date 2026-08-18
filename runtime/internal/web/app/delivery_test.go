package app

import (
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestStartDeliveryRunCreatesManifest(t *testing.T) {
	root := t.TempDir()
	toolkit := testToolkitRoot(t)
	if err := StartDeliveryRun(root, toolkit, "web-probe", "probe goal", "goal-delivery"); err != nil {
		t.Fatal(err)
	}
	manifest := state.ManifestPath(root, "web-probe")
	run, err := state.Load(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if run.CurrentNode != "intake" {
		t.Fatalf("node = %q", run.CurrentNode)
	}
	if run.Status != state.StatusAwaitingHuman {
		t.Fatalf("status = %q", run.Status)
	}
}

func TestStartDeliveryRunRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	toolkit := testToolkitRoot(t)
	if err := StartDeliveryRun(root, toolkit, "dup", "one", "goal-delivery"); err != nil {
		t.Fatal(err)
	}
	if err := StartDeliveryRun(root, toolkit, "dup", "two", "goal-delivery"); err == nil {
		t.Fatal("expected duplicate error")
	}
}
