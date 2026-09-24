package docmeta

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// forbiddenStateKey matches a run-manifest key written as a mapping key, so
// that "checks" here is manifest.json's checks map and not the ordinary
// English word appearing in a sentence like "run the checks before shipping".
// Anchored to the start of a line (YAML) or after a quote (JSON), each
// followed by a colon, is what a key declaration looks like and prose never
// does.
var forbiddenStateKey = regexp.MustCompile(`(?m)^\s*"?(currentNode|checks|maxTransitions)"?\s*:`)

// checkNoGraphState reports run/graph state leaking into a docs/ deliverable.
// Status/content separation (AGENTS.md "Generated docs location") means a
// generated doc never carries currentNode, checks, or maxTransitions - those
// stay in .agent-state/runs/.../manifest.json, never in docs/.
//
// Fenced code blocks are excluded: a spec explaining the run-state schema
// (docs/2026-07-29/loop-graph-runtime designed this exact schema) legitimately
// shows a currentNode/checks example inside a ```json fence, and that is
// documentation, not a leak.
func checkNoGraphState(rel string, raw []byte) *Issue {
	m := forbiddenStateKey.FindSubmatch(StripFencedCode(raw))
	if m == nil {
		return nil
	}
	return &Issue{
		Path:    rel,
		Message: "run/graph state key " + string(m[1]) + " must not appear under docs/; it belongs in .agent-state/runs/.../manifest.json",
	}
}

// StripFencedCode blanks the content of every ``` ... ``` block, keeping line
// numbers stable and leaving the fence markers themselves so nothing outside
// a block shifts position.
func StripFencedCode(raw []byte) []byte {
	lines := strings.Split(string(raw), "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			lines[i] = ""
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// Issue is one layout or metadata problem under docs/.
type Issue struct {
	Path    string
	Message string
}

func (i Issue) String() string {
	return i.Path + ": " + i.Message
}

// CheckWorkspace reports forbidden flat deliverables and missing front matter
// on versioned SPEC/PLAN/TASKS files.
func CheckWorkspace(root string) ([]Issue, error) {
	docsRoot := filepath.Join(root, workspace.DocsDirName)
	entries, err := os.ReadDir(docsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var issues []Issue
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		dir := filepath.Join(docsRoot, name)
		if validate.Date(name) {
			more, err := checkVersionedDate(dir, name)
			if err != nil {
				return nil, err
			}
			issues = append(issues, more...)
			continue
		}
		if validate.Slug(name) {
			issues = append(issues, checkFlatSlug(dir, name)...)
		}
	}
	return issues, nil
}

func checkFlatSlug(dir, slug string) []Issue {
	var issues []Issue
	for _, stem := range []string{"SPEC.md", "PLAN.md", "TASKS.md", "tasks.json"} {
		path := filepath.Join(dir, stem)
		if _, err := os.Stat(path); err == nil {
			issues = append(issues, Issue{
				Path:    filepath.ToSlash(path),
				Message: "flat docs/" + slug + "/" + stem + " is forbidden after migrate; run vibe-agent migrate docs-tmp",
			})
		}
	}
	return issues
}

func checkVersionedDate(dateDir, date string) ([]Issue, error) {
	var issues []Issue
	slugs, err := os.ReadDir(dateDir)
	if err != nil {
		return nil, err
	}
	for _, slugEntry := range slugs {
		if !slugEntry.IsDir() || !validate.Slug(slugEntry.Name()) {
			continue
		}
		versions, err := os.ReadDir(filepath.Join(dateDir, slugEntry.Name()))
		if err != nil {
			return nil, err
		}
		for _, verEntry := range versions {
			if !verEntry.IsDir() {
				continue
			}
			rev := filepath.Join(dateDir, slugEntry.Name(), verEntry.Name())
			more, err := checkRevision(rev, date)
			if err != nil {
				return nil, err
			}
			issues = append(issues, more...)
		}
	}
	return issues, nil
}

func checkRevision(dir, date string) ([]Issue, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(dir, name)
		rel := filepath.ToSlash(path)
		switch {
		case name == "SPEC.md" || name == "PLAN.md" || name == "TASKS.md" || name == "tasks.json":
			issues = append(issues, Issue{
				Path:    rel,
				Message: "undated basename under a versioned tree; rename to {STEM}-" + date + ".{ext}",
			})
		case isDatedProse(name, date):
			raw, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return nil, err
			}
			if _, err := ParseFrontMatter(raw); err != nil {
				issues = append(issues, Issue{
					Path:    rel,
					Message: "front matter: " + err.Error(),
				})
			}
			if issue := checkNoGraphState(rel, raw); issue != nil {
				issues = append(issues, *issue)
			}
		}
	}
	return issues, nil
}

func isDatedProse(name, date string) bool {
	for _, stem := range []string{"SPEC", "PLAN", "TASKS"} {
		if name == stem+"-"+date+".md" {
			return true
		}
	}
	return strings.HasSuffix(name, "-"+date+".md") && (strings.HasPrefix(name, "SPEC-") || strings.HasPrefix(name, "PLAN-") || strings.HasPrefix(name, "TASKS-"))
}
