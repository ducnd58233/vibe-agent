package harness

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

// The slop audit shipped with a scanner, a CLI and a test suite, and nothing
// called it. `grep -rn "vibe-agent slop" .ai-agents/` returned nothing: no
// command, no skill, no graph node, no entry in vibe-checks.yaml. A capability
// nobody invokes is indistinguishable from one that does not exist.
//
// It is bound in four places now. Three of them are deliberate acts a person
// takes: a check in vibe-checks.yaml, a verifier node in the delivery graph,
// and a required read in /review and /ship. This is the fourth, and the only
// one that happens without being asked for.
//
// Which makes it the one that has to be cheapest and quietest:
//
//   - one file, the one just written, never a directory walk;
//   - advisory only, never a refusal, because the write already happened;
//   - medium severity and above. The low-severity rules are dominated by
//     duplicate_line, which fires on ordinary repeated table rows and test
//     fixtures. Reporting those on every edit is how a channel gets ignored,
//     and the channel is shared with the credential and design-token guards,
//     which must not be ignored.
//
// The full audit still reports everything. What is withheld here is the
// interruption, not the finding.

// slopAdviceTimeout bounds the scan. This runs after every file write, so the
// ceiling matters more than completeness: a scan that cannot finish one file in
// this long has nothing useful to say about it either.
const slopAdviceTimeout = 3 * time.Second

// slopAdviceLimit is how many findings are shown before the rest are counted.
//
// A file with fifteen new problems does not need fifteen lines to say so, and
// the advice channel is shared.
const slopAdviceLimit = 5

// slopAdvice reports what the audit found in the file that was just written.
func slopAdvice(req Request, file subject) string {
	path := file.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(req.WorkspaceRoot, filepath.FromSlash(file.Path))
	}

	ctx, cancel := context.WithTimeout(context.Background(), slopAdviceTimeout)
	defer cancel()

	// One worker. The pool exists for auditing a repository; here there is a
	// single file, and spawning workers to scan it costs more than it saves.
	report := slopaudit.Audit(ctx, path, slopaudit.Options{Workers: 1})

	notable := notableFindings(report.Findings)
	if len(notable) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("[slop] %s scores %d/100.", file.Path, report.Score))
	shown := min(len(notable), slopAdviceLimit)
	for _, finding := range notable[:shown] {
		lines = append(lines, fmt.Sprintf("  %s:%d %s - %s",
			file.Path, finding.Line, finding.Rule, finding.Message))
	}
	if rest := len(notable) - shown; rest > 0 {
		lines = append(lines, fmt.Sprintf("  and %d more; run `vibe-agent slop audit %s` for all of them.",
			rest, file.Path))
	}
	lines = append(lines, "  Advisory. The write already happened, and nothing here refused it.")
	return strings.Join(lines, "\n")
}

// scanErrorRule is what the auditor reports when it could not read a file at
// all, as a high-severity finding on line 1.
//
// It is a fact about the scan and not about the code, and it must not reach
// this channel: a file removed between the write and the hook would otherwise
// be announced as a high-severity slop problem, naming a file that is not
// there. Measured, not assumed - auditing a missing path returns
// "scan_error ... The system cannot find the file specified" with a score of 8.
//
// The audit still reports it. A person running `vibe-agent slop audit` on a
// path that does not exist should be told so; a person editing a file should
// not be told it in the guard channel.
const scanErrorRule = "scan_error"

// notableFindings drops what is not worth an interruption: the severities below
// medium, and the findings that are about the scan rather than the code.
func notableFindings(findings []domain.Finding) []domain.Finding {
	kept := make([]domain.Finding, 0, len(findings))
	for _, finding := range findings {
		if finding.Rule == scanErrorRule {
			continue
		}
		if finding.Severity == domain.SeverityMedium || finding.Severity == domain.SeverityHigh {
			kept = append(kept, finding)
		}
	}
	return kept
}
