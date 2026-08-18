package catalog

import (
	"path"
	"regexp"
	"strings"
)

var (
	tableRule = regexp.MustCompile(`^\|[\s\-:|]+\|$`)
	linkRe    = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
)

type tableRow struct {
	cells []string
}

func tableRows(text string) []tableRow {
	var rows []tableRow
	seenRule := false
	for _, line := range strings.Split(text, "\n") {
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
		rows = append(rows, tableRow{cells: cells})
	}
	return rows
}

func linkTarget(cell string) string {
	match := linkRe.FindStringSubmatch(cell)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func assetSlug(target string) string {
	clean := path.Clean(target)
	base := path.Base(clean)
	if base == "SKILL.md" {
		return path.Base(path.Dir(clean))
	}
	return strings.TrimSuffix(base, path.Ext(base))
}
