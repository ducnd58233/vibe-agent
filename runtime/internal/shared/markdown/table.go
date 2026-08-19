package markdown

import (
	"path"
	"regexp"
	"strings"
)

var (
	tableRule = regexp.MustCompile(`^\|[\s\-:|]+\|$`)
	linkRe    = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
)

// TableRow is one body row from the first markdown table in a document.
type TableRow struct {
	Line  int
	Cells []string
}

// ParseFirstTable returns body rows after the header rule of the first pipe table.
func ParseFirstTable(text string) []TableRow {
	var rows []TableRow
	seenRule := false
	for number, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimRight(line, " \t\r")
		if !strings.HasPrefix(trimmed, "|") {
			if seenRule && len(rows) > 0 {
				break
			}
			continue
		}
		if tableRule.MatchString(trimmed) {
			seenRule = true
			continue
		}
		if !seenRule {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, TableRow{Line: number + 1, Cells: cells})
	}
	return rows
}

// LinkTarget returns the URL or path from the first markdown link in cell.
func LinkTarget(cell string) string {
	match := linkRe.FindStringSubmatch(cell)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// LinkTargets returns every link destination in cell, in order.
func LinkTargets(cell string) []string {
	matches := linkRe.FindAllStringSubmatch(cell, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			out = append(out, strings.TrimSpace(match[1]))
		}
	}
	return out
}

// AssetSlug is the routable name a link target points at: a directory with
// SKILL.md, or the basename without extension for a single .md file.
func AssetSlug(target string) string {
	clean := path.Clean(target)
	base := path.Base(clean)
	if base == "SKILL.md" {
		return path.Base(path.Dir(clean))
	}
	return strings.TrimSuffix(base, path.Ext(base))
}
