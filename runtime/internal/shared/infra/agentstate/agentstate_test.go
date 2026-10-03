package agentstate

import (
	"os"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestGetNeverCreatesTheDatabase(t *testing.T) {
	root := t.TempDir()
	value, ok, err := Get(t.Context(), root, "ns", "k")
	if err != nil || ok || value != "" {
		t.Fatalf("Get = %q %v %v", value, ok, err)
	}
	if _, err := os.Stat(workspace.MemoryDBPath(root)); err == nil {
		t.Fatal("a read created memory.db")
	}
}

func TestSetOverwritesAndNamespacesAreSeparate(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	for _, step := range []struct{ ns, key, value string }{
		{"cursor", "last_node", "build"},
		{"cursor", "last_node", "ship"},
		{"other", "last_node", "unrelated"},
	} {
		if err := Set(ctx, root, step.ns, step.key, step.value); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if got, ok, _ := Get(ctx, root, "cursor", "last_node"); !ok || got != "ship" {
		t.Errorf("cursor = %q %v", got, ok)
	}
	if got, _, _ := Get(ctx, root, "other", "last_node"); got != "unrelated" {
		t.Errorf("other = %q", got)
	}
	if err := Delete(ctx, root, "cursor", "last_node"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := Get(ctx, root, "cursor", "last_node"); ok {
		t.Error("key survived Delete")
	}
}
