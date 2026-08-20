package deadcode

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

// The tool prints one line per finding and has no machine-readable output, so
// parsing it is the part that can silently stop working. A scanner that quietly
// returns nothing looks exactly like a clean tree.
func TestParseTurnsToolOutputIntoFindings(t *testing.T) {
	out := strings.Join([]string{
		`internal\fetch\fetch.go:40:6: unreachable func: AssetDir`,
		"internal/web/view/graph.go:223:6: unreachable func: FormatGraphEdges",
		"some unrelated line the tool printed",
	}, "\n")

	findings := parse(out, "runtime")
	if len(findings) != 2 {
		t.Fatalf("parsed %d findings, want 2: %+v", len(findings), findings)
	}

	first := findings[0]
	if first.Line != 40 {
		t.Errorf("line = %d, want 40", first.Line)
	}
	if first.Rule != Rule {
		t.Errorf("rule = %q, want %q", first.Rule, Rule)
	}
	if first.Severity != Severity {
		t.Errorf("severity = %q, want %q", first.Severity, Severity)
	}
	if !strings.Contains(first.Message, "AssetDir") {
		t.Errorf("message does not name the symbol: %q", first.Message)
	}
	// Paths are reported relative to the module, and a finding has to point at
	// something a reader can open from the target.
	if !strings.HasPrefix(filepath.ToSlash(first.Path), "runtime/") {
		t.Errorf("path = %q, want it rooted at the module", first.Path)
	}
	if strings.Contains(first.Path, `\`) {
		t.Errorf("path is not in slash form: %q", first.Path)
	}
}

// The finding has to say what to do, or the next move is to work around it.
func TestAFindingSaysWhatToDo(t *testing.T) {
	findings := parse("internal/a/a.go:1:1: unreachable func: Thing", "runtime")
	if len(findings) != 1 {
		t.Fatalf("parsed %d findings, want 1", len(findings))
	}
	for _, want := range []string{"remove it", "deadcode.Kept", "reason"} {
		if !strings.Contains(findings[0].Message, want) {
			t.Errorf("message does not contain %q: %q", want, findings[0].Message)
		}
	}
}

// A seam kept on purpose must not be reported, or the check fires on every run
// and gets switched off.
func TestAnAllowlistedSymbolIsNotReported(t *testing.T) {
	out := "internal/agent/infra/anthropic/anthropic.go:86:6: unreachable func: New"
	if findings := parse(out, "runtime"); len(findings) != 0 {
		t.Errorf("an allowlisted symbol was reported: %+v", findings)
	}
}

// The allowlist matches a path and a symbol together. The same name somewhere
// else is a different function and stays reported.
func TestTheAllowlistDoesNotMatchTheSameNameElsewhere(t *testing.T) {
	out := "internal/somewhere/else.go:12:1: unreachable func: New"
	if findings := parse(out, "runtime"); len(findings) != 1 {
		t.Errorf("the allowlist matched on name alone: %+v", findings)
	}
}

// Every allowlist entry carries a reason and matches one symbol in one file. An
// entry without a reason is a suppression nobody can argue with later; one
// without both halves is a rule that silences more than it was meant to.
//
// An empty allowlist passes this trivially, which is correct: the failure being
// guarded against is a bad entry, not the absence of entries.
func TestEveryAllowlistEntryIsNarrowAndExplained(t *testing.T) {
	for _, keep := range Kept {
		if strings.TrimSpace(keep.Reason) == "" {
			t.Errorf("%s %s is allowlisted with no reason", keep.Suffix, keep.Symbol)
		}
		if keep.Suffix == "" || keep.Symbol == "" {
			t.Errorf("an allowlist entry matches too broadly: %+v", keep)
		}
	}
}

// slop audit walks whatever target it is given, in any language. A Go-only
// analyser that failed on a directory with no module would be a bug in the
// auditor rather than a finding about the code.
func TestScanSkipsATargetWithNoGoModule(t *testing.T) {
	result, err := NewScanner("").Scan(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("scanning a non-Go target errored: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("a target with no go.mod produced findings: %+v", result.Findings)
	}
}

// The slop score is a density, and this is not a density question.
//
// One unreachable exported function is a defect whether the repository is a
// thousand lines or a hundred thousand, and at the second size the density
// rounds it to nothing. Relying on the score would have produced a check that
// passes precisely where it matters most: on a large codebase.
func TestTheScoreAloneCannotCarryThisFinding(t *testing.T) {
	findings := parse("internal/a/a.go:1:1: unreachable func: Thing", "runtime")

	// The fact this design turns on, asserted rather than assumed: at repository
	// scale the density rounds one finding away entirely.
	if scored := domain.Score(findings, 100000); scored != 0 {
		t.Errorf("Score = %d over 100k lines, want 0; if density now carries a "+
			"single finding, the presence rule below can be reconsidered", scored)
	}
	if len(Findings(findings)) != 1 {
		t.Error("presence is the verdict, and it did not report one")
	}
}

// Presence is the verdict, so the caller needs the findings picked out reliably.
func TestFindingsPicksOutOnlyThisScannersFindings(t *testing.T) {
	mixed := append(
		parse("internal/a/a.go:1:1: unreachable func: Thing", "runtime"),
		domain.Finding{Path: "b.go", Line: 2, Rule: "duplicate_line", Severity: domain.SeverityLow},
	)
	mine := Findings(mixed)
	if len(mine) != 1 {
		t.Fatalf("picked %d findings, want 1: %+v", len(mine), mine)
	}
	if mine[0].Rule != Rule {
		t.Errorf("rule = %q, want %q", mine[0].Rule, Rule)
	}
	if len(Findings(nil)) != 0 {
		t.Error("a nil report produced findings")
	}
}
