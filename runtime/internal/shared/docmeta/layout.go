package docmeta

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/validate"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

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
				Message: "flat docs/" + slug + "/" + stem + " is forbidden; write docs/<date>/<slug>/<version>/",
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
