package graphroute

import (
	"strings"
	"testing"
)

func TestDetectRoutesObjectives(t *testing.T) {
	cases := []struct {
		goal     string
		workflow Workflow
		task     bool // the task_required flag
		strong   bool
	}{
		// Code, and the default.
		{"Add retry ceiling to the webhook dispatcher", WorkflowDelivery, false, true},
		{"Fix the bug in the login endpoint", WorkflowDelivery, false, true},
		{"Refactor the python module that parses invoices", WorkflowDelivery, false, true},
		{"Make the thing faster", WorkflowDelivery, false, false},
		{"Add an email field to the signup form", WorkflowDelivery, false, false},
		// Mixed: a code change and a non-code deliverable.
		{"Add retry to the webhook API and draft an announcement for the customer team", WorkflowDelivery, true, true},
		{"Implement the export endpoint and write a report on how it works", WorkflowDelivery, true, true},
		// Non-code task.
		{"Draft a reply to the customer about the late invoice", WorkflowTask, false, false},
		{"Summarize the vendor contract and write a memo for the team", WorkflowTask, false, true},
		{"Reconcile the expenses spreadsheet against the bank csv", WorkflowTask, false, false},
		// Research and experiment.
		{"Research chunking strategies for retrieval on legal PDFs", WorkflowResearch, false, false},
		{"Literature review of quantization, what does the evidence say", WorkflowResearch, false, true},
		{"Run an ablation and benchmark of embedding models", WorkflowExperiment, false, true},
		{"Compare models on recall with an experiment", WorkflowExperiment, false, true},
		// Research with code words stays on delivery, research is a phase there.
		{"Add a small inference module: research quantization, spec, implement, test", WorkflowDelivery, false, true},
		// Learning.
		{"Teach me linear algebra before my exam in December", WorkflowTutor, false, false},
		{"I want to learn Spanish, quiz me every week and make a study plan", WorkflowTutor, false, true},
		{"Teach me to write a REST API in python", WorkflowTutor, false, false},
	}
	for _, tc := range cases {
		got := Detect(tc.goal)
		if len(got.Ambiguous) > 0 {
			t.Errorf("%q: unexpectedly ambiguous: %v", tc.goal, got.Ambiguous)
			continue
		}
		if got.Workflow != tc.workflow {
			t.Errorf("%q: workflow %s, want %s (%s)", tc.goal, got.Workflow, tc.workflow, got.Reason)
		}
		if got.Flags["task_required"] != tc.task {
			t.Errorf("%q: task_required %v, want %v", tc.goal, got.Flags["task_required"], tc.task)
		}
		if got.Strong != tc.strong {
			t.Errorf("%q: strong %v, want %v (%s)", tc.goal, got.Strong, tc.strong, got.Reason)
		}
		if got.Reason == "" {
			t.Errorf("%q: no reason given", tc.goal)
		}
	}
}

func TestDetectAsksWhenResearchAndTaskCollide(t *testing.T) {
	got := Detect("Research the competitors and draft a report for the team")
	if len(got.Ambiguous) != 2 {
		t.Fatalf("want a two-way tie, got %+v", got)
	}
}

func TestDetectIsDeterministic(t *testing.T) {
	const goal = "Add retry to the webhook API and draft an announcement for the customer team"
	first := Detect(goal)
	for i := 0; i < 20; i++ {
		again := Detect(goal)
		if again.Workflow != first.Workflow || again.Reason != first.Reason {
			t.Fatalf("detection changed between calls: %+v then %+v", first, again)
		}
	}
}

func TestResolveExplicitWorkflowSkipsDetection(t *testing.T) {
	got, err := Params{Command: CmdGoal, Workflow: WorkflowTask, Goal: "Add retry to the webhook API"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.GraphID != GraphTask || got.Reason != "named on the command line" {
		t.Errorf("got %+v", got)
	}
}

func TestResolveImplicitReadsTheObjectiveAndSaysWhy(t *testing.T) {
	got, err := Params{Command: CmdGoal, Goal: "Summarize the vendor contract and write a memo for the team"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.GraphID != GraphTask || !strings.Contains(got.Reason, "non-code deliverable") {
		t.Errorf("got %+v", got)
	}
}

func TestResolveMixedSetsTheFlagOnlyOnDelivery(t *testing.T) {
	got, err := Params{Command: CmdGoal, Goal: "Implement the export endpoint and write a report on how it works"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.GraphID != GraphDelivery || !got.Flags["task_required"] {
		t.Errorf("got %+v", got)
	}

	// The flag is a guard of the delivery graph, so it is not set on another.
	got, err = Params{Command: CmdGoal, Workflow: WorkflowResearch, WithTask: true, Goal: "Research chunking"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got.Flags["task_required"] {
		t.Errorf("task_required set on %s", got.GraphID)
	}
}

func TestResolveWithTaskForcesTheFlagOnDelivery(t *testing.T) {
	got, err := Params{Command: CmdGoal, Workflow: WorkflowDelivery, WithTask: true, Goal: "Ship the thing"}.Resolve()
	if err != nil || !got.Flags["task_required"] {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestResolveAutoActsOnlyOnAStrongNonDeliverySignal(t *testing.T) {
	// One weak word is not enough to start an unattended run on a different graph.
	_, err := Params{Command: CmdAuto, Goal: "Draft a reply to the customer about the late invoice"}.Resolve()
	if err == nil || !strings.Contains(err.Error(), "too little") {
		t.Errorf("want a refusal to guess, got %v", err)
	}
	// A strong one is enough.
	got, err := Params{Command: CmdAuto, Goal: "Summarize the vendor contract and write a memo for the team"}.Resolve()
	if err != nil || got.GraphID != GraphTask {
		t.Errorf("got %+v, %v", got, err)
	}
	// Delivery, the default, never needs the confidence.
	got, err = Params{Command: CmdAuto, Goal: "Make the thing faster"}.Resolve()
	if err != nil || got.GraphID != GraphDelivery {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestResolveAutoRefusesATutor(t *testing.T) {
	for _, p := range []Params{
		{Command: CmdAuto, Workflow: WorkflowTutor, Goal: "linear algebra"},
		{Command: CmdAuto, Goal: "Teach me linear algebra; I want to learn it, quiz me weekly"},
	} {
		_, err := p.Resolve()
		if err == nil || !strings.Contains(err.Error(), "goal tutor") {
			t.Errorf("%+v: want a refusal that names goal tutor, got %v", p, err)
		}
	}
}

func TestResolveAsksWhenTheObjectiveCouldBeTwoKinds(t *testing.T) {
	_, err := Params{Command: CmdGoal, Goal: "Research the competitors and draft a report for the team"}.Resolve()
	if err == nil || !strings.Contains(err.Error(), "Say which") {
		t.Errorf("want a question, got %v", err)
	}
}

func TestParseWorkflow(t *testing.T) {
	for _, word := range []string{"delivery", "research", "experiment", "task", "tutor", "TASK"} {
		if _, ok := ParseWorkflow(word); !ok {
			t.Errorf("%q was not recognised", word)
		}
	}
	for _, word := range []string{"", "tasks", "build", "goal"} {
		if _, ok := ParseWorkflow(word); ok {
			t.Errorf("%q was recognised", word)
		}
	}
	if GraphForWorkflow(WorkflowExperiment) != GraphResearcher || GraphForWorkflow(WorkflowTutor) != GraphStudy {
		t.Error("workflow to graph mapping is wrong")
	}
}
