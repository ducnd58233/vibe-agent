package main

import (
	"os"
	"path/filepath"
	"testing"
)

// toolkitWithFixtures builds a toolkit tree holding the given assets and a
// routing-evals table made of the supplied rows.
func toolkitWithFixtures(t *testing.T, table string, assets ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range assets {
		writeConfig(t, root, filepath.FromSlash(rel), "# asset\n")
	}
	writeConfig(t, root, filepath.Join(".ai-agents", "references", "routing-evals.md"),
		"# Routing evals\n\n"+
			"| User intent | Expected family | Expected asset |\n"+
			"|-------------|-----------------|----------------|\n"+
			table)
	return root
}

func problemsFrom(t *testing.T, root string) int {
	t.Helper()
	report := &diagnostics{}
	checkRoutingEvals(report, root)
	return report.problems
}

// The family column is the claim the old link check could not see. Moving an
// asset between folders leaves the link resolving and the family wrong, so the
// fixture keeps passing while describing a route that no longer exists.
func TestAFamilyThatDisagreesWithTheFolderIsCaught(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Review a diff before merge | command | [`code-review`](../skills/code-review/SKILL.md) |\n",
		".ai-agents/skills/code-review/SKILL.md")

	if problemsFrom(t, root) == 0 {
		t.Error("a row filed under command while pointing into skills/ was accepted")
	}
}

// Two rows claiming one intent make the expected route ambiguous: whichever the
// eval reads second silently decides what the first one meant.
func TestARepeatedIntentIsCaught(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Write a failing test first | skill | [`tdd`](../skills/tdd/SKILL.md) |\n"+
			"| write a FAILING test first | skill | [`tdd`](../skills/tdd/SKILL.md) |\n",
		".ai-agents/skills/tdd/SKILL.md")

	if problemsFrom(t, root) == 0 {
		t.Error("the same intent was accepted twice; case alone should not make it a new row")
	}
}

func TestAnExpectedAssetThatDoesNotExistIsCaught(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Ship a risky release | skill | [`gone`](../skills/gone/SKILL.md) |\n")

	if problemsFrom(t, root) == 0 {
		t.Error("a fixture pointing at a missing asset was accepted")
	}
}

// A fixture that spells out its own answer tests string matching rather than
// routing, which is the failure the file's own phrasing rule exists to prevent.
func TestAnIntentNamingItsOwnAssetIsCaught(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Apply token-efficient-execution to this loop | skill "+
			"| [`x`](../skills/token-efficient-execution/SKILL.md) |\n",
		".ai-agents/skills/token-efficient-execution/SKILL.md")

	if problemsFrom(t, root) == 0 {
		t.Error("an intent quoting its asset's slug was accepted")
	}
}

// The other half of that rule. A one-word slug is usually the plain verb a
// person would reach for, so matching on it would reject the very phrasing the
// rule asks for. Only a hyphenated slug is evidence that a row named its answer.
func TestAOneWordSlugIsNotMistakenForNamingTheAsset(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Review a diff across five axes before merge | command | [`review.md`](../commands/review.md) |\n",
		".ai-agents/commands/review.md")

	if problems := problemsFrom(t, root); problems != 0 {
		t.Errorf("a naturally phrased intent was rejected for containing the verb "+
			"its command is named after (%d problems)", problems)
	}
}

// A table that holds together reports nothing, so the checks stay readable when
// somebody adds a row.
func TestAWellFormedTablePasses(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Write a failing test first, then implement | skill | [`x`](../skills/test-driven-development/SKILL.md) |\n"+
			"| Audit toolkit asset health | command | [`doctor.md`](../commands/doctor.md) |\n"+
			"| Security review of auth and secrets | agent | [`y`](../agents/security-auditor.md) |\n",
		".ai-agents/skills/test-driven-development/SKILL.md",
		".ai-agents/commands/doctor.md",
		".ai-agents/agents/security-auditor.md")

	if problems := problemsFrom(t, root); problems != 0 {
		t.Errorf("a well-formed table reported %d problems", problems)
	}
}

// A consumer repo can mount the toolkit without the fixtures. Reporting a
// problem there would fail a build over a file the repo never claimed to have.
func TestAToolkitWithoutFixturesIsNotAProblem(t *testing.T) {
	if problems := problemsFrom(t, t.TempDir()); problems != 0 {
		t.Errorf("a toolkit with no routing-evals.md reported %d problems", problems)
	}
}

// Everything above proves the rules work against tables written to exercise
// them. This proves they hold for the fixtures this repository actually ships,
// which is the file a person edits and the checker exists to protect.
//
// It runs here rather than as a CI step because `doctor` also checks the
// vibe-agent on PATH, and a build machine has no reason to have one installed.
// Calling the one check directly gets the real file gated by the suite that
// already gates the build.
func TestThisRepositorysOwnFixturesPass(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, ".ai-agents")); err != nil {
		t.Skip("runtime is checked out without the toolkit assets beside it")
	}

	if problems := problemsFrom(t, root); problems != 0 {
		t.Errorf(".ai-agents/references/routing-evals.md has %d problem(s); "+
			"run `vibe-agent doctor` to see them", problems)
	}
}

// Coverage counts assets that exist, not rows. A fixture naming an asset that
// was deleted must not still count toward the number somebody is watching.
func TestCoverageCountsOnlyAssetsOnDisk(t *testing.T) {
	root := toolkitWithFixtures(t,
		"| Write a failing test first | skill | [`x`](../skills/tdd/SKILL.md) |\n",
		".ai-agents/skills/tdd/SKILL.md",
		".ai-agents/skills/unrouted/SKILL.md")

	// Two skills exist and one is routed to; the check reports rather than fails,
	// so what is asserted here is that reporting it is not itself a problem.
	if problems := problemsFrom(t, root); problems != 0 {
		t.Errorf("an uncovered asset was treated as a failure (%d problems)", problems)
	}
}
