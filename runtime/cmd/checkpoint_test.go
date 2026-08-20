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
