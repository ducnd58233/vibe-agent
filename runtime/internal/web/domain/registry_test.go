package domain

import (
	"path/filepath"
	"testing"
)

func TestRegistryIDResolveRoundTrip(t *testing.T) {
	reg := NewRegistry("/tmp/a", []string{"/tmp/b"})
	want := filepath.Clean("/tmp/b")
	id := reg.ID(want)
	root, ok := reg.Resolve(id)
	if !ok || root != want {
		t.Fatalf("resolve = %q ok = %v want %q", root, ok, want)
	}
}

func TestNewRegistryDedupesRoots(t *testing.T) {
	reg := NewRegistry("/tmp/a", []string{"/tmp/a", "/tmp/b"})
	if len(reg.Roots) != 2 {
		t.Fatalf("roots = %v", reg.Roots)
	}
}
