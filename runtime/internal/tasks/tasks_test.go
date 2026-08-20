package tasks

import (
	"strings"
	"testing"
)

const valid = `{
  "schemaVersion": 1,
  "slug": "demo",
  "tasks": [
    {"id": "T1", "title": "first", "status": "done"},
    {"id": "T2", "title": "second", "status": "queued", "dependsOn": ["T1"]}
  ]
}`

func TestParseAcceptsAWellFormedList(t *testing.T) {
	file, err := Parse([]byte(valid))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(file.Tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(file.Tasks))
	}
	if remaining := file.Remaining(); len(remaining) != 1 || remaining[0].ID != "T2" {
		t.Errorf("remaining = %+v, want only T2", remaining)
	}
}

// An empty list would end a run rather than describe one, which is the
// expensive direction to be wrong in.
func TestParseRefusesAnEmptyList(t *testing.T) {
	raw := strings.Replace(valid, `"tasks": [`, `"tasks": [], "ignored": [`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("an empty task list parsed")
	}
}

func TestParseRefusesAnUnknownStatus(t *testing.T) {
	raw := strings.Replace(valid, `"status": "queued"`, `"status": "in-progress"`, 1)
	_, err := Parse([]byte(raw))
	if err == nil {
		t.Fatal("a misspelled status parsed")
	}
	if !strings.Contains(err.Error(), "in_progress") {
		t.Errorf("error = %q, want it to name the states it accepts", err)
	}
}

func TestParseRefusesADuplicateID(t *testing.T) {
	raw := strings.Replace(valid, `"id": "T2"`, `"id": "T1"`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("two tasks shared an id; the second would silently redefine the first")
	}
}

// A dependency on a task that is not in the list describes an ordering nothing
// can satisfy.
func TestParseRefusesADependencyOnAMissingTask(t *testing.T) {
	raw := strings.Replace(valid, `["T1"]`, `["T9"]`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("a dependency on a task outside the list parsed")
	}
}

func TestParseRefusesAnotherSchemaVersion(t *testing.T) {
	raw := strings.Replace(valid, `"schemaVersion": 1`, `"schemaVersion": 2`, 1)
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("an unknown schemaVersion parsed")
	}
}

// Only done and canceled settle a task. A blocked task is still in scope, which
// is the whole point of having the state.
func TestOnlyDoneAndCanceledSettleATask(t *testing.T) {
	settled := map[Status]bool{
		StatusQueued: false, StatusInProgress: false, StatusBlocked: false,
		StatusDone: true, StatusCanceled: true,
	}
	for status, want := range settled {
		if got := status.Settled(); got != want {
			t.Errorf("%s settled = %t, want %t", status, got, want)
		}
	}
}
