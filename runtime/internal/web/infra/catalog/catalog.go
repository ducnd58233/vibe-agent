package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Family groups toolkit assets exposed in the composer catalog.
type Family string

const (
	FamilyCommand Family = "command"
	FamilySkill   Family = "skill"
)

// Entry is one routable command or skill row from a ROUTER table.
type Entry struct {
	Family      Family
	Slug        string
	Description string
	Insert      string
}

// Index holds parsed catalog rows for commands and skills.
type Index struct {
	Commands []Entry
	Skills   []Entry
}

// Load reads command and skill ROUTER tables from the toolkit root.
func Load(toolkitRoot string) (Index, error) {
	root := filepath.Clean(toolkitRoot)
	commands, err := loadRouter(filepath.Join(root, ".ai-agents", "commands", "ROUTER.md"), FamilyCommand)
	if err != nil {
		return Index{}, fmt.Errorf("commands router: %w", err)
	}
	skills, err := loadRouter(filepath.Join(root, ".ai-agents", "skills", "ROUTER.md"), FamilySkill)
	if err != nil {
		return Index{}, fmt.Errorf("skills router: %w", err)
	}
	return Index{Commands: commands, Skills: skills}, nil
}

func loadRouter(path string, family Family) ([]Entry, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, row := range tableRows(string(raw)) {
		if len(row.cells) < 2 {
			continue
		}
		target := linkTarget(row.cells[1])
		if target == "" {
			continue
		}
		slug := assetSlug(target)
		if slug == "" {
			continue
		}
		insert := "@" + slug
		if family == FamilyCommand {
			insert = "/" + slug
		}
		out = append(out, Entry{
			Family:      family,
			Slug:        slug,
			Description: row.cells[0],
			Insert:      insert,
		})
	}
	return out, nil
}

// SearchCommands returns command rows matching q (slug or description).
func (idx Index) SearchCommands(q string) []Entry {
	return search(idx.Commands, q)
}

// SearchSkills returns skill rows matching q.
func (idx Index) SearchSkills(q string) []Entry {
	return search(idx.Skills, q)
}

func search(items []Entry, q string) []Entry {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return items
	}
	var out []Entry
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Slug), q) ||
			strings.Contains(strings.ToLower(item.Description), q) {
			out = append(out, item)
		}
	}
	return out
}
