package hosts

import (
	"strings"
	"testing"
)

func TestInventoryMissingBinary(t *testing.T) {
	lookPath = func(name string) (string, error) {
		return "", errNotFound(name)
	}
	t.Cleanup(func() { lookPath = defaultLookPath })

	entry := Inventory()[0]
	if entry.OnPath {
		t.Fatal("expected missing host to be off PATH")
	}
	if entry.Reason == "" || !strings.Contains(entry.Reason, "not on PATH") {
		t.Fatalf("reason = %q", entry.Reason)
	}
}

func TestInventoryPresentBinary(t *testing.T) {
	lookPath = func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	}
	t.Cleanup(func() { lookPath = defaultLookPath })

	for _, entry := range Inventory() {
		if !entry.OnPath {
			t.Fatalf("%s should be on PATH", entry.ID)
		}
		if entry.Reason != "" {
			t.Fatalf("reason should be empty when present: %q", entry.Reason)
		}
	}
}

func TestCatalogListsFourHosts(t *testing.T) {
	if got := len(Catalog()); got != 4 {
		t.Fatalf("catalog = %d hosts, want 4", got)
	}
}

func TestEvalHostAcceptsCursorAlias(t *testing.T) {
	byCursor, ok := EvalHost("cursor")
	if !ok {
		t.Fatal("cursor")
	}
	byBinary, ok := EvalHost("cursor-agent")
	if !ok {
		t.Fatal("cursor-agent should resolve; the composer posts the catalog id")
	}
	if byCursor.Binary != "cursor-agent" || byBinary.Binary != "cursor-agent" {
		t.Fatalf("cursor=%q cursor-agent=%q", byCursor.Binary, byBinary.Binary)
	}
}

func errNotFound(name string) error {
	return &pathError{name: name}
}

type pathError struct{ name string }

func (e *pathError) Error() string { return e.name + ": not found" }

var defaultLookPath = lookPath
