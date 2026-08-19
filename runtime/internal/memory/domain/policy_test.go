package domain

import (
	"strings"
	"testing"
)

func TestFilterRejectsCredentialViaRedact(t *testing.T) {
	candidate := Record{
		WorkspaceID: "ws1",
		Kind:        KindEpisodic,
		Content:     "Deploy token github_pat_" + strings.Repeat("a", 24),
		Confidence:  0.9,
		Status:      StatusProposed,
		SourceType:  SourceCommandResult,
		Evidence:    []string{"command exited 1"},
	}
	decision := Filter{}.Decide(candidate)
	if decision.Verdict != VerdictReject || !strings.Contains(decision.Reason, "secret") {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestFilterRejectsTransientContent(t *testing.T) {
	candidate := Record{
		WorkspaceID: "ws1",
		Kind:        KindEpisodic,
		Content:     "Blocked on pending CI before merge",
		Confidence:  0.9,
		Status:      StatusProposed,
		SourceType:  SourceCommandResult,
		Evidence:    []string{"gh pr checks still running"},
	}
	decision := Filter{}.Decide(candidate)
	if decision.Verdict != VerdictReject {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestFilterRejectsHedgedContent(t *testing.T) {
	candidate := Record{
		WorkspaceID: "ws1",
		Kind:        KindEpisodic,
		Content:     "I believe Redis is probably required here",
		Confidence:  0.9,
		Status:      StatusProposed,
		SourceType:  SourceCommandResult,
		Evidence:    []string{"integration test failed once"},
	}
	decision := Filter{}.Decide(candidate)
	if decision.Verdict != VerdictReject {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestFilterStoresEvidenceBackedObservation(t *testing.T) {
	candidate := Record{
		WorkspaceID: "ws1",
		Kind:        KindEpisodic,
		Content:     "Integration tests require Redis on localhost:6379.",
		Confidence:  0.95,
		Status:      StatusProposed,
		SourceType:  SourceCommandResult,
		Evidence:    []string{"make integration-test failed with connection refused"},
	}
	decision := Filter{}.Decide(candidate)
	if decision.Verdict != VerdictStore {
		t.Fatalf("decision = %+v", decision)
	}
}
