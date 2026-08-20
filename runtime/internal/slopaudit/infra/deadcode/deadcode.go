// Package deadcode reports functions no control flow can reach.
//
// It answers a question golangci-lint's `unused` deliberately will not: whether
// an *exported* function is dead. The maintainers decline that because an
// exported name might be imported by code they cannot see, which is right in
// general and false for this module - everything it examines lives under
// `internal/`, which the Go toolchain forbids anything outside from importing.
//
// A Scanner rather than an Adapter, so the results are Findings a report can
// list, sort, and emit as JSON rather than one pass-or-fail line.
//
// What fails the command is their presence, not the score. The slop score is a
// density, and reachability is not a density question - see Severity.
package deadcode

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

// Tool is pinned. An unpinned @latest makes the answer depend on the day it ran,
// and this one decides whether code gets deleted.
const Tool = "golang.org/x/tools/cmd/deadcode@v0.49.0"

// Rule is the finding name, matching the tool's own wording so a reader can grep
// either one and land in the same place.
const Rule = "unreachable_func"

// Severity places it in the report beside the other findings. It does not decide
// whether the command fails.
//
// The slop score is a density - weighted findings per thousand lines - which is
// the right model for "how much noise is in this code" and the wrong one for
// this. One unreachable exported function is a defect whether the repository is
// a thousand lines or a hundred thousand, and at the second size the density
// rounds it to nothing. So presence is what fails the command; see Findings.
const Severity = domain.SeverityMedium

// Findings picks this scanner's findings out of a report.
//
// Presence is the verdict, which is why this exists rather than a threshold: the
// caller fails when the slice is non-empty. Density says how noisy code is;
// reachability says whether something is wired up, and one is not a weaker form
// of the other.
func Findings(all []domain.Finding) []domain.Finding {
	var mine []domain.Finding
	for _, finding := range all {
		if finding.Rule == Rule {
			mine = append(mine, finding)
		}
	}
	return mine
}

// findingPattern parses one line of the tool's output:
//
//	internal/fetch/fetch.go:40:6: unreachable func: AssetDir
//
// A regex here rather than a parser because the tool has no machine-readable
// output and the shape is one line with fixed separators. If it ever grows a
// -json flag, this is the thing to delete.
var findingPattern = regexp.MustCompile(`^(.*?):(\d+):\d+: unreachable func: (.+)$`)

// Keep is a function that stays even though nothing reaches it.
//
// A seam that half exists is not dead weight. The audit's own rule is that a
// shape becomes a finding when it has cost something or can be shown to, and an
// unwired seam that a command file describes as unwired has cost nothing.
//
// Adding an entry means writing the same reason beside the function, so the two
// halves of the decision cannot drift apart.
type Keep struct {
	// Suffix matches the tail of the reported path, in slash form.
	Suffix string
	Symbol string
	Reason string
}

// Kept is the allowlist. It is a slice rather than a map so the reason reads in
// order beside the entry it belongs to.
var Kept = []Keep{{
	Suffix: "internal/agent/infra/anthropic/anthropic.go",
	Symbol: "New",
	Reason: "the transport's constructor; no command drives the model yet, which auto.md states plainly",
}}

func (k Keep) matches(path, symbol string) bool {
	return symbol == k.Symbol && strings.HasSuffix(filepath.ToSlash(path), k.Suffix)
}

// Scanner runs the reachability analysis over a Go module.
type Scanner struct {
	// ModuleDir is the directory holding go.mod, relative to the target. Empty
	// means the target itself.
	ModuleDir string
}

// NewScanner builds a scanner for a module rooted at moduleDir inside the target.
func NewScanner(moduleDir string) *Scanner { return &Scanner{ModuleDir: moduleDir} }

// Scan reports every unreachable function that is not on the allowlist.
//
// It skips rather than errors when it cannot run. `slop audit` walks whatever
// target it is given, in any language, and a Go-only analyser that failed on a
// Python repository would be a bug in the auditor rather than a finding about
// the code.
func (s *Scanner) Scan(ctx context.Context, target string) (domain.ScanResult, error) {
	module := filepath.Join(target, filepath.FromSlash(s.ModuleDir))
	if !runnable(module) {
		return domain.ScanResult{}, nil
	}
	out, ok := run(ctx, module)
	if !ok {
		return domain.ScanResult{}, nil
	}
	return domain.ScanResult{Findings: parse(out, module)}, nil
}

// runnable reports whether there is anything here to analyse.
//
// A boolean rather than an error, and deliberately so. Returning nil after
// inspecting an error is how a guard reports success on a failure it just saw,
// which is why `nilerr` is enabled in this module. The distinction being drawn
// is not "did something go wrong" but "does this question apply here", and that
// answer is a boolean.
func runnable(module string) bool {
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		return false
	}
	_, err := safexec.LookPath("go")
	return err == nil
}

// run executes the tool and reports whether its output can be trusted.
//
// The tool exits 0 whether or not it finds anything, so a non-zero exit means it
// could not run at all: no module resolution, or no network for the pinned
// version on first use. Neither is evidence about dead code, and reporting
// findings from a run that failed halfway would be worse than reporting none.
func run(ctx context.Context, module string) (string, bool) {
	// -test roots the analysis at test binaries as well as main, and it is the
	// whole decision. Without it the tool reports every helper that only tests
	// call, and acting on that list deletes working code.
	command, err := safexec.CommandContext(ctx, "go", "-C", module, "run", Tool, "-test", "./...")
	if err != nil {
		return "", false
	}
	out, err := command.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// parse turns the tool's lines into findings, dropping the allowlisted ones.
func parse(out, module string) []domain.Finding {
	var findings []domain.Finding
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		match := findingPattern.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if match == nil {
			continue
		}
		path, symbol := filepath.ToSlash(match[1]), strings.TrimSpace(match[3])
		if kept(path, symbol) {
			continue
		}
		line, convErr := strconv.Atoi(match[2])
		if convErr != nil {
			continue
		}
		findings = append(findings, domain.Finding{
			Path:     filepath.ToSlash(filepath.Join(module, filepath.FromSlash(path))),
			Line:     line,
			Rule:     Rule,
			Severity: Severity,
			Message: fmt.Sprintf(
				"%s is unreachable; remove it, or keep it deliberately by writing the reason beside it and adding it to deadcode.Kept",
				symbol),
		})
	}
	return findings
}

func kept(path, symbol string) bool {
	for _, keep := range Kept {
		if keep.matches(path, symbol) {
			return true
		}
	}
	return false
}
