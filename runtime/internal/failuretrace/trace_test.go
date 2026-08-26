package failuretrace

import (
	"strings"
	"testing"
)

func TestParseRequiresRefineTarget(t *testing.T) {
	body := `run_id: r1
slug: demo
failed_node: test
failure_class: test
symptom: unit red
events_ref: events.ndjson#3
`
	if _, err := Parse(strings.NewReader(body)); err == nil {
		t.Fatal("expected missing refine_target to fail")
	}
}

func TestParseHappyPath(t *testing.T) {
	body := `# Failure TRACE
run_id: r1
slug: demo
failed_node: test
failure_class: test
symptom: unit red
events_ref: events.ndjson#3
refine_target: build
`
	tr, err := Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if tr.RefineTarget != "build" || tr.FailureClass != "test" || tr.FailedNode != "test" {
		t.Fatalf("unexpected trace: %+v", tr)
	}
}

func TestDefaultRefineTarget(t *testing.T) {
	if got := DefaultRefineTarget("ambiguity"); got != "plan" {
		t.Fatalf("ambiguity -> %q, want plan", got)
	}
	if got := DefaultRefineTarget("test"); got != "build" {
		t.Fatalf("test -> %q, want build", got)
	}
}
