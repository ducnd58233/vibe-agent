package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The routing evals pair a user intent with the asset the router should select.
// Until now the only thing enforced about them was that the link resolves, which
// scripts/check-ai-agents-routers.sh did in eight lines of bash. That catches a
// renamed file and nothing else, so a row could name the wrong family, repeat an
// intent another row already claims, or phrase the intent as the asset's own
// name - and each of those leaves a fixture agreeing with itself while testing
// nothing.
//
// The family column is the sharpest of them. A row reading
//
//	| Audit toolkit asset health | command | [`doctor.md`](../commands/doctor.md) |
//
// is a claim about where the asset lives. Move doctor.md into skills/ and the
// link check follows it happily while the fixture goes on asserting a family
// that is no longer true, describing a route nobody can take.
//
// This lives in the runtime rather than beside the other scripts because doctor
// already reads .ai-agents: checkGraphs loads the graph directory from the same
// toolkit root. Go also gets it the two things the script checkers do not have -
// tests in the suite that gates the build, and delivery to a consumer through
// the binary, with no Python toolchain to install first.

// familyHome maps the family column to the folder that family lives in.
var familyHome = map[string]string{
	"skill":         "skills",
	"agent":         "agents",
	"command":       "commands",
	"reference":     "references",
	"graph":         "graphs",
	"hook":          "hooks",
	"stack-profile": "stack-profiles",
}

// countedFamilies are the ones whose assets can be enumerated one per file or
// one per directory, so coverage means something. The rest are still validated.
var countedFamilies = []string{"skill", "agent", "command"}

// notAnAsset are the router scaffolding files, which no intent routes to.
var notAnAsset = map[string]bool{"ROUTER.md": true, "TEMPLATE.md": true, "README.md": true}

var (
	evalLink  = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	tableRule = regexp.MustCompile(`^\|[\s\-:|]+\|$`)
)

// tableRow is one body row of a markdown table, carrying the line it came from
// so a problem can name a place a person can open.
type tableRow struct {
	Line  int
	Cells []string
}

// tableRows returns the body rows of the first markdown table in text.
//
// It reads a table the way the bash checker does: skip to the |---| rule, then
// take pipe rows until the table ends. Anything after the first table is prose.
func tableRows(text string) []tableRow {
	var rows []tableRow
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
		rows = append(rows, tableRow{Line: number + 1, Cells: cells})
	}
	return rows
}

// assetSlug is the name a link target points at, for both layouts an asset uses:
// a directory holding SKILL.md, or a single .md file.
func assetSlug(target string) string {
	clean := path.Clean(target)
	base := path.Base(clean)
	if base == "SKILL.md" {
		return path.Base(path.Dir(clean))
	}
	return strings.TrimSuffix(base, path.Ext(base))
}

// routableAssets is every asset of a family present on disk.
func routableAssets(toolkitRoot, family string) map[string]bool {
	found := map[string]bool{}
	home := filepath.Join(toolkitRoot, ".ai-agents", familyHome[family])
	entries, err := os.ReadDir(home)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if family == "skill" {
			if !entry.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(home, name, "SKILL.md")); err == nil {
				found[name] = true
			}
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || notAnAsset[name] {
			continue
		}
		found[strings.TrimSuffix(name, ".md")] = true
	}
	return found
}

// checkRoutingEvals validates the fixture table and reports intent coverage.
//
// Problems are grouped by rule rather than by row. Twenty-one fixtures against
// five rules is a hundred verdicts, and a wall of "ok" is the same as no output:
// one line per rule, naming every offender, is what a person can act on.
//
// Coverage is reported and never fails. The file's own rule is to add a row
// "when you add a routable asset users will ask for by intent", which is a
// judgment about what users ask for, and no check can make it. The number is
// here to be watched rather than enforced.
func checkRoutingEvals(report *diagnostics, toolkitRoot string) {
	fixtures := filepath.Join(toolkitRoot, ".ai-agents", "references", "routing-evals.md")
	raw, err := os.ReadFile(filepath.Clean(fixtures))
	if err != nil {
		return // optional: a consumer repo may mount the toolkit without it
	}

	var malformed, missing, misfiled, repeated, selfNaming []string
	seen := map[string]int{}
	covered := map[string]map[string]bool{}
	for family := range familyHome {
		covered[family] = map[string]bool{}
	}
	rows := 0

	for _, row := range tableRows(string(raw)) {
		at := fmt.Sprintf("line %d", row.Line)

		if len(row.Cells) != 3 || row.Cells[0] == "" {
			malformed = append(malformed, fmt.Sprintf("%s (%d columns, want intent, family, asset)",
				at, len(row.Cells)))
			continue
		}
		rows++
		intent, family, cell := row.Cells[0], row.Cells[1], row.Cells[2]

		key := strings.ToLower(intent)
		if first, ok := seen[key]; ok {
			repeated = append(repeated, fmt.Sprintf("%s repeats line %d", at, first))
		} else {
			seen[key] = row.Line
		}

		if _, ok := familyHome[family]; !ok {
			malformed = append(malformed, fmt.Sprintf("%s unknown family %q", at, family))
			continue
		}

		targets := evalLink.FindAllStringSubmatch(cell, -1)
		if len(targets) != 1 {
			malformed = append(malformed, fmt.Sprintf("%s holds %d links, want 1", at, len(targets)))
			continue
		}
		target := targets[0][1]

		if _, err := os.Stat(filepath.Join(filepath.Dir(fixtures), filepath.FromSlash(target))); err != nil {
			missing = append(missing, fmt.Sprintf("%s -> %s", at, target))
			continue
		}

		if !strings.Contains(path.Clean(target), "/"+familyHome[family]+"/") {
			misfiled = append(misfiled, fmt.Sprintf("%s says %s but %s is not under %s/",
				at, family, target, familyHome[family]))
			continue
		}

		slug := assetSlug(target)
		covered[family][slug] = true

		// Only a hyphenated slug is evidence. A one-word slug like "review" is
		// also the plain verb a person would use, so matching on it would fail
		// the very phrasing the rule asks for.
		if strings.Contains(slug, "-") && strings.Contains(key, slug) {
			selfNaming = append(selfNaming, fmt.Sprintf("%s names %q", at, slug))
		}
	}

	report.check(fmt.Sprintf("routing evals parse (%d fixtures)", rows),
		len(malformed) == 0, strings.Join(malformed, "; "))
	report.check("routing evals: every expected asset exists",
		len(missing) == 0, strings.Join(missing, "; "))
	report.check("routing evals: family matches the folder the asset lives in",
		len(misfiled) == 0, strings.Join(misfiled, "; "))
	report.check("routing evals: no intent is claimed twice",
		len(repeated) == 0, strings.Join(repeated, "; "))
	report.check("routing evals: no intent names its own asset",
		len(selfNaming) == 0, strings.Join(selfNaming, "; "))

	var parts []string
	hit, total := 0, 0
	for _, family := range countedFamilies {
		onDisk := routableAssets(toolkitRoot, family)
		found := 0
		for slug := range covered[family] {
			if onDisk[slug] {
				found++
			}
		}
		hit += found
		total += len(onDisk)
		parts = append(parts, fmt.Sprintf("%s %d/%d", familyHome[family], found, len(onDisk)))
	}
	sort.Strings(parts)
	if total > 0 {
		fmt.Printf("  note  routing eval coverage %d/%d (%s)\n", hit, total, strings.Join(parts, ", "))
	}
}
