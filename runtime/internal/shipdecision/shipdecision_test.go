package shipdecision

import (
	"strings"
	"testing"
)

func TestParseGO(t *testing.T) {
	input := `Ship Decision: GO
Specialist: code-reviewer -> PASS
Specialist: security-auditor -> PASS
Specialist: test-engineer -> PASS
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !d.GO {
		t.Errorf("GO = false, want true")
	}
	if len(d.Blockers) != 0 {
		t.Errorf("Blockers = %v, want none", d.Blockers)
	}
	want := []Specialist{
		{Name: "code-reviewer", Passed: true},
		{Name: "security-auditor", Passed: true},
		{Name: "test-engineer", Passed: true},
	}
	if len(d.Specialists) != len(want) {
		t.Fatalf("Specialists = %v, want %v", d.Specialists, want)
	}
	for i, s := range d.Specialists {
		if s != want[i] {
			t.Errorf("Specialists[%d] = %v, want %v", i, s, want[i])
		}
	}
}

func TestParseNOGO(t *testing.T) {
	input := `Ship Decision: NO-GO
BLOCKER: missing test coverage on the new branch
BLOCKER: SQL built by string concatenation in internal/foo
Specialist: code-reviewer -> FAIL
Specialist: security-auditor -> FAIL
Specialist: test-engineer -> PASS
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.GO {
		t.Errorf("GO = true, want false")
	}
	if len(d.Blockers) != 2 {
		t.Errorf("Blockers = %v, want 2 entries", d.Blockers)
	}
}

func TestParseGOWithNoSpecialists(t *testing.T) {
	// ship.md's own triviality rule can skip fan-out entirely; a GO with zero
	// specialist lines must still parse, not be treated as malformed.
	d, err := Parse(strings.NewReader("Ship Decision: GO\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !d.GO || len(d.Specialists) != 0 {
		t.Errorf("Decision = %+v, want GO with no specialists", d)
	}
}

func TestParseRejectsGOWithBlockers(t *testing.T) {
	_, err := Parse(strings.NewReader("Ship Decision: GO\nBLOCKER: leftover from a bad merge\n"))
	if err == nil {
		t.Fatal("Parse succeeded on GO with a blocker, want an error")
	}
}

func TestParseRejectsNOGOWithoutBlockers(t *testing.T) {
	_, err := Parse(strings.NewReader("Ship Decision: NO-GO\n"))
	if err == nil {
		t.Fatal("Parse succeeded on NO-GO with no blockers, want an error")
	}
}

func TestParseRejectsMissingDecisionLine(t *testing.T) {
	_, err := Parse(strings.NewReader("Specialist: code-reviewer -> PASS\n"))
	if err == nil {
		t.Fatal("Parse succeeded with no Ship Decision line, want an error")
	}
}

func TestParseRejectsDuplicateDecisionLine(t *testing.T) {
	_, err := Parse(strings.NewReader("Ship Decision: GO\nShip Decision: GO\n"))
	if err == nil {
		t.Fatal("Parse succeeded with a duplicate decision line, want an error")
	}
}

func TestParseRejectsUnknownVerdict(t *testing.T) {
	_, err := Parse(strings.NewReader("Ship Decision: MAYBE\n"))
	if err == nil {
		t.Fatal("Parse succeeded on an unknown verdict, want an error")
	}
}

func TestParseRejectsMalformedSpecialistLine(t *testing.T) {
	_, err := Parse(strings.NewReader("Ship Decision: GO\nSpecialist: code-reviewer WEIRD\n"))
	if err == nil {
		t.Fatal("Parse succeeded on a malformed specialist line, want an error")
	}
}

func TestParseRejectsUnrecognizedLine(t *testing.T) {
	_, err := Parse(strings.NewReader("Ship Decision: GO\nsome prose that is not a recognized line\n"))
	if err == nil {
		t.Fatal("Parse succeeded on an unrecognized line, want an error")
	}
}

func TestParseIgnoresBlankLines(t *testing.T) {
	input := "Ship Decision: GO\n\n\nSpecialist: code-reviewer -> PASS\n\n"
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !d.GO || len(d.Specialists) != 1 {
		t.Errorf("Decision = %+v, want GO with one specialist", d)
	}
}
