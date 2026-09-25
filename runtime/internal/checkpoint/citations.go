package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/citations"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	sharedworkspace "github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// CitationCheck reports the cited URLs that do not resolve. Injected so tests
// never touch the network; nil means the public-internet checker.
type CitationCheck func(ctx context.Context, urls []string) []citations.Failure

// citationTargets are the nodes that produce a research digest, per graph.
var citationTargets = map[string]struct{ node, filename string }{
	"goal-delivery":       {node: "auto_research", filename: "RESEARCH-${date}.md"},
	"researcher-delivery": {node: "literature", filename: "RESEARCH-${date}.md"},
}

// checkCitations refuses to leave a research node while the digest cites a URL
// that does not resolve. It fails closed: offline, the run stays put and the
// error names every URL that could not be reached.
func checkCitations(ctx context.Context, workspaceRoot string, run *state.Run, check CitationCheck) error {
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
	urls := citations.Extract(raw)
	if len(urls) == 0 {
		return nil
	}
	if check == nil {
		check = citations.Default().Check
	}
	failures := check(ctx, urls)
	if len(failures) == 0 {
		return nil
	}
	lines := make([]string, len(failures))
	for i, failure := range failures {
		lines[i] = "  " + failure.String()
	}
	return fmt.Errorf("%s at %s cites %d URL(s) that do not resolve; fix or remove each before continuing:\n%s",
		filepath.Base(path), run.CurrentNode, len(failures), strings.Join(lines, "\n"))
}
