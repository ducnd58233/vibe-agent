package checkpoint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/calc"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// checkCalcs refuses to leave a research node while the digest logs a
// calculation that does not recompute. It reads the same file checkCitations
// does, and it needs no network, so unlike that check it never fails closed for
// a reason other than a wrong figure.
//
// A digest with no calc block passes: the rule is that a figure the digest
// computed is logged and checkable, and a digest that computed nothing owes
// nothing.
func checkCalcs(workspaceRoot string, run *state.Run) error {
	target, ok := citationTargets[run.GraphID]
	if !ok || target.node != run.CurrentNode || run.Date == "" || run.Version < 1 {
		return nil
	}
	path := filepath.Join(sharedworkspace.DocsDirAt(workspaceRoot, run.Date, run.Slug, run.Version),
		strings.ReplaceAll(target.filename, "${date}", run.Date))
	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	_, _, issues := calc.CheckMarkdown(raw)
	if len(issues) == 0 {
		return nil
	}
	lines := make([]string, len(issues))
	for i, issue := range issues {
		lines[i] = "  " + issue.String()
	}
	return fmt.Errorf("%s at %s logs %d calculation(s) that do not recompute; fix each figure, or the line, before continuing:\n%s",
		filepath.Base(path), run.CurrentNode, len(issues), strings.Join(lines, "\n"))
}
