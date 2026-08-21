package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// Acceptance heading and checkbox shapes used in TASKS prose. Kept narrow on
// purpose: agents already write this section by the planning skill template.
var (
	acceptanceHeading = regexp.MustCompile(`(?i)^\*{0,2}acceptance criteria:?\*{0,2}\s*$`)
	checkboxLine      = regexp.MustCompile(`(?i)^\s*[-*]\s+\[([ xX])\]\s+\S`)
	sectionHeading    = regexp.MustCompile(`^#{2,3}\s+`)
)

// ProsePath is the TASKS markdown beside a slug's task list, or "" when none.
func ProsePath(workspaceRoot, slug string) string {
	entry, err := runpath.Resolve(workspaceRoot, slug)
	date := ""
	if err == nil {
		date = entry.Date
		dir := workspace.DocsDirAt(workspaceRoot, entry.Date, entry.Slug, entry.Version)
		if name, nameErr := workspace.DocsArtifact("TASKS", entry.Date); nameErr == nil {
			candidate := filepath.Join(dir, name)
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
		}
		legacy := filepath.Join(dir, "TASKS.md")
		if _, statErr := os.Stat(legacy); statErr == nil {
			return legacy
		}
	}
	if file, loadErr := Load(workspaceRoot, slug); loadErr == nil && date == "" {
		date = file.Date
	}
	dir := workspace.DocsDir(workspaceRoot, slug)
	if date != "" {
		if name, nameErr := workspace.DocsArtifact("TASKS", date); nameErr == nil {
			candidate := filepath.Join(dir, name)
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
		}
	}
	legacy := filepath.Join(dir, "TASKS.md")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return ""
}

// LoadProse reads TASKS markdown for a slug. Missing prose is ("", nil).
func LoadProse(workspaceRoot, slug string) (string, error) {
	path := ProsePath(workspaceRoot, slug)
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// IncompleteDone lists tasks marked done whose acceptance checkboxes in prose
// are not all checked. Those tasks must not settle a run: status done alone is
// how agents previously closed a task while leaving AC boxes open.
func (f *File) IncompleteDone(prose string) []Task {
	var out []Task
	for _, task := range f.Tasks {
		if task.Status != StatusDone {
			continue
		}
		if reason := AcceptanceIncomplete(prose, task.ID); reason != "" {
			out = append(out, task)
		}
	}
	return out
}

// RemainingAgainstProse reports tasks still in scope: unsettled statuses, plus
// done tasks whose acceptance checkboxes are incomplete (or whose prose cannot
// prove them). Empty prose treats every done task as incomplete: checkboxes are
// required for done to settle.
func (f *File) RemainingAgainstProse(prose string) []Task {
	var out []Task
	for _, task := range f.Tasks {
		switch {
		case !task.Status.Settled():
			out = append(out, task)
		case task.Status == StatusDone && AcceptanceIncomplete(prose, task.ID) != "":
			out = append(out, task)
		}
	}
	return out
}

// AcceptanceIncomplete explains why a done task's AC section is not complete.
// Empty string means every checkbox under Acceptance criteria is checked.
func AcceptanceIncomplete(prose, taskID string) string {
	section := TaskSection(prose, taskID)
	if section == "" {
		return fmt.Sprintf("task %s has no TASKS prose section", taskID)
	}
	boxes := AcceptanceCheckboxes(section)
	if len(boxes) == 0 {
		return fmt.Sprintf("task %s has no acceptance-criteria checkboxes", taskID)
	}
	for _, box := range boxes {
		if !box.Checked {
			return fmt.Sprintf("task %s has unchecked acceptance criteria", taskID)
		}
	}
	return ""
}

// Checkbox is one markdown task item under Acceptance criteria.
type Checkbox struct {
	Checked bool
	Text    string
}

// TaskSection returns one "## T<id>:" / "### T<id>:" body through the next
// same-level-or-higher heading.
func TaskSection(markdown, taskID string) string {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return ""
	}
	lines := strings.Split(markdown, "\n")
	start := -1
	prefixDepth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "##") {
			continue
		}
		depth := headingDepth(trimmed)
		title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if strings.HasPrefix(title, id+":") || strings.HasPrefix(title, id+" ") {
			start = i
			prefixDepth = depth
			break
		}
	}
	if start == -1 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !sectionHeading.MatchString(trimmed) {
			continue
		}
		if headingDepth(trimmed) <= prefixDepth {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

// AcceptanceCheckboxes returns checkbox items under the Acceptance criteria
// block inside a task section.
func AcceptanceCheckboxes(taskSection string) []Checkbox {
	lines := strings.Split(taskSection, "\n")
	start := -1
	for i, line := range lines {
		if acceptanceHeading.MatchString(strings.TrimSpace(line)) {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return nil
	}
	var out []Checkbox
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if sectionHeading.MatchString(trimmed) {
			break
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "**") && !strings.HasPrefix(lower, "**acceptance") {
			// Next labelled subsection (Verification, Delivery branch, Files).
			break
		}
		match := checkboxLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		text := strings.TrimSpace(line)
		if idx := strings.Index(text, "]"); idx >= 0 && idx+1 < len(text) {
			text = strings.TrimSpace(text[idx+1:])
		}
		out = append(out, Checkbox{
			Checked: match[1] == "x" || match[1] == "X",
			Text:    text,
		})
	}
	return out
}

func headingDepth(line string) int {
	n := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		n++
	}
	return n
}
