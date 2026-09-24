// Package docgrounding checks that a generated doc's backtick-quoted file
// paths actually resolve in the repo tree, as a warning inside /review's
// local pass - not a new vibe-checks.yaml gate. It cannot detect every
// fabricated claim (AGENTS.md "no source for model assertion" applies here
// too), only the mechanically checkable slice: a path a doc cites that does
// not exist.
package docgrounding

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/docmeta"
)

// Issue is one dangling path reference found in a doc.
type Issue struct {
	Path string // as written in the doc
	Line int    // 1-indexed
}

// backtickSpan matches a single-backtick inline code span.
var backtickSpan = regexp.MustCompile("`([^`\n]+)`")

// futureMarker matches wording that explicitly acknowledges a path does not
// exist yet, so a planned deliverable is not flagged as a fabrication.
var futureMarker = regexp.MustCompile(`(?i)\b(planned|not yet|future|will add|to be (added|built|written))\b`)

// Check reads mdPath and returns one Issue per backtick-quoted candidate path
// that does not resolve under workspaceRoot. A candidate is a span containing
// "/" with no spaces and no "<...>" placeholder segment (docs/<date>/<slug>/
// is a documented pattern, not a literal path). A span on a line matching
// futureMarker is not flagged. Fenced code blocks are excluded, the same way
// docmeta.CheckWorkspace excludes them from the graph-state scan - both a
// schema example and a hoped-for CLI transcript belong there, not among
// claims the doc asserts are already true.
func Check(workspaceRoot, mdPath string) ([]Issue, error) {
	raw, err := os.ReadFile(filepath.Clean(mdPath))
	if err != nil {
		return nil, err
	}
	stripped := docmeta.StripFencedCode(raw)

	var issues []Issue
	for lineNum, line := range strings.Split(string(stripped), "\n") {
		if futureMarker.MatchString(line) {
			continue
		}
		for _, m := range backtickSpan.FindAllStringSubmatch(line, -1) {
			candidate := m[1]
			if !looksLikePath(candidate) {
				continue
			}
			if pathExists(workspaceRoot, candidate) {
				continue
			}
			issues = append(issues, Issue{Path: candidate, Line: lineNum + 1})
		}
	}
	return issues, nil
}

// looksLikePath reports whether a backtick span is shaped like a file or
// directory path worth checking, rather than a flag, identifier, slash
// command, glob pattern, or URL that happens to be in backticks too.
func looksLikePath(s string) bool {
	if !strings.Contains(s, "/") {
		return false // "currentNode", "--slug": not path-shaped
	}
	if strings.ContainsAny(s, "<> ") {
		return false // template placeholder or a prose fragment, not a path
	}
	if strings.Contains(s, "://") {
		return false // URL
	}
	if strings.HasPrefix(s, "-") {
		return false // a flag, e.g. "-C runtime"
	}
	if strings.ContainsAny(s, "*{}") || strings.Contains(s, "...") {
		return false // a glob, brace-expansion, or ellipsis pattern - not a literal path
	}
	if strings.HasPrefix(s, "/") && !strings.Contains(s[1:], "/") {
		return false // a slash command, e.g. "/vibe-auto" or "/review" - single segment after the slash
	}
	return true
}

// pathExists checks candidate, and candidate with a trailing "/" or
// punctuation trimmed, against the workspace tree.
//
// Refuses anything that would resolve outside workspaceRoot: the candidate
// comes from doc text (agent- or human-authored, but not trusted input), and
// joining it unchecked would let a "../../../etc/passwd"-shaped reference
// Stat a real file outside the repo and be reported as grounded.
func pathExists(workspaceRoot, candidate string) bool {
	trimmed := strings.TrimRight(candidate, "/.,;:")
	if trimmed == "" {
		return true // nothing left to check
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return false
	}
	joined, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(trimmed)))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false // escapes workspaceRoot
	}
	_, err = os.Stat(joined)
	return err == nil
}
