package checkpoint

import (
	"context"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/citations"
	"github.com/ducnd58233/vibe-agent/runtime/internal/loop"
)

// fakeCitations fails every URL containing "dead" and records what it saw.
func fakeCitations(seen *[]string) CitationCheck {
	return func(_ context.Context, urls []string) []citations.Failure {
		*seen = append(*seen, urls...)
		var failures []citations.Failure
		for _, url := range urls {
			if strings.Contains(url, "dead") {
				failures = append(failures, citations.Failure{URL: url, Reason: "404 Not Found"})
			}
		}
		return failures
	}
}

func applyWithCitations(t *testing.T, root string, check CitationCheck) (*Result, error) {
	return Apply(t.Context(), Request{
		WorkspaceRoot: root, GraphDir: graphDir, Slug: "demo",
		Outcome: loop.Outcome{}, Now: at(), Citations: check,
	})
}

func TestADeadCitationKeepsTheRunAtAutoResearch(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "Live: https://arxiv.org/abs/2604.03173\nDead: https://dead.example/paper\n")

	var seen []string
	result, err := applyWithCitations(t, root, fakeCitations(&seen))
	if err == nil {
		t.Fatalf("a digest citing a dead URL advanced: %+v", result.Transition)
	}
	if !strings.Contains(err.Error(), "https://dead.example/paper") {
		t.Errorf("error does not name the dead URL: %v", err)
	}
	if len(seen) != 2 {
		t.Errorf("checker saw %v, want both cited URLs", seen)
	}
}

func TestLiveCitationsLetTheRunLeaveAutoResearch(t *testing.T) {
	root := t.TempDir()
	atAutoResearch(t, root)
	writeResearchArtifact(t, root, "Live: https://arxiv.org/abs/2604.03173\n")

	var seen []string
	result, err := applyWithCitations(t, root, fakeCitations(&seen))
	if err != nil {
		t.Fatalf("live citations were refused: %v", err)
	}
	if result.Transition == nil || result.Transition.From != "auto_research" {
		t.Errorf("run did not advance: %+v", result.Transition)
	}
}
