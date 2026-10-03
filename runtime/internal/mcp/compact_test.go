package mcp

import (
	"strings"
	"testing"
)

func TestCompactEvidenceBoundsWhatAHitCosts(t *testing.T) {
	long := strings.Repeat("the build failed with a connection refused error ", 20)
	evidence := []string{long, "second", "third", "fourth", "fifth"}
	got := compactEvidence(evidence)
	if len(got) != maxEvidenceItems {
		t.Fatalf("kept %d items, want %d", len(got), maxEvidenceItems)
	}
	if len(got[0]) > maxEvidenceChars+3 || !strings.HasSuffix(got[0], "...") {
		t.Errorf("long evidence was not cut: %d chars", len(got[0]))
	}
	if got[1] != "second" {
		t.Errorf("short evidence was altered: %q", got[1])
	}
	if len(evidence) != 5 {
		t.Error("the caller's slice was modified")
	}
}
