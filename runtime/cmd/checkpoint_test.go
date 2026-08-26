package main

import (
	"strings"
	"testing"
)

// A class describes a failure. Offering one without a failure to attach it to
// would record a class against a passing step, which is worse than recording
// nothing: it puts a failure category on evidence that succeeded.
func TestAClassWithoutABlockerIsRefused(t *testing.T) {
	err := checkpointCommand([]string{
		"--workspace", t.TempDir(), "--slug", "demo", "--class", "tool",
	})
	if err == nil {
		t.Fatal("a class was accepted with nothing failing")
	}
	if !strings.Contains(err.Error(), "--blocker") {
		t.Errorf("error = %q, want it to name --blocker", err)
	}
}

// A blocker without a class cannot be counted or audited; that was the gap
// research measured (near-zero class fill). The CLI refuses rather than warn.
func TestABlockerWithoutAClassIsRefused(t *testing.T) {
	err := checkpointCommand([]string{
		"--workspace", t.TempDir(), "--slug", "demo", "--blocker", "tests red",
	})
	if err == nil {
		t.Fatal("a blocker was accepted with no class")
	}
	if !strings.Contains(err.Error(), "--class") {
		t.Errorf("error = %q, want it to name --class", err)
	}
}

func TestAnUnknownFailureClassIsRefused(t *testing.T) {
	err := checkpointCommand([]string{
		"--workspace", t.TempDir(), "--slug", "demo",
		"--blocker", "tests red", "--class", "flaky",
	})
	if err == nil {
		t.Fatal("an unknown class was accepted")
	}
	if !strings.Contains(err.Error(), "failure class") {
		t.Errorf("error = %q, want it to name failure class", err)
	}
}
