package graphroute

import "testing"

func TestGraphForCommands(t *testing.T) {
	t.Parallel()
	if GraphFor(CmdGoal) != GraphDelivery {
		t.Fatal("goal should use delivery graph")
	}
	if GraphFor(CmdResearch) != GraphResearcher {
		t.Fatal("research should use researcher graph")
	}
}

func TestResolveDerivesSlug(t *testing.T) {
	t.Parallel()
	got, err := Params{Command: CmdAuto, Goal: "Add a retry ceiling to the webhook dispatcher"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "add-retry-ceiling-webhook" {
		t.Fatalf("slug = %q", got.Slug)
	}
	if got.GraphID != GraphDelivery {
		t.Fatalf("graph = %q", got.GraphID)
	}
}

func TestResolveRefusesNonEnglishObjectiveWithoutExplicitSlug(t *testing.T) {
	t.Parallel()
	_, err := Params{Command: CmdAuto, Goal: "Kiểm tra runtime trước khi merge"}.Resolve()
	if err == nil {
		t.Fatal("expected refusal for a non-English objective with no explicit slug")
	}
}

func TestResolveAcceptsNonEnglishObjectiveWithExplicitSlug(t *testing.T) {
	t.Parallel()
	got, err := Params{Command: CmdAuto, Goal: "Kiểm tra runtime trước khi merge", Slug: "check-runtime-before-merge"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "check-runtime-before-merge" {
		t.Fatalf("slug = %q", got.Slug)
	}
}

func TestResolveResearchWorkflow(t *testing.T) {
	t.Parallel()
	got, err := Params{
		Command:  CmdAuto,
		Workflow: WorkflowResearch,
		Goal:     "Compare RAG chunking for legal QA",
	}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.GraphID != GraphResearcher {
		t.Fatalf("graph = %q", got.GraphID)
	}
}
