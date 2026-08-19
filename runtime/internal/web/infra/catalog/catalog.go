package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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

// LoadForWorkspace loads the toolkit router catalog, then merges in command
// and skill entries discovered in consumer workspace and global tool install
// directories. Each GenAI host defines its own locations for skills and
// commands; this function scans all of them.
func LoadForWorkspace(workspaceRoot, toolkitRoot string) (Index, error) {
	idx, err := Load(toolkitRoot)
	if err != nil {
		return Index{}, err
	}

	seenCmd := make(map[string]struct{}, len(idx.Commands))
	for _, c := range idx.Commands {
		seenCmd[c.Slug] = struct{}{}
	}
	for _, dir := range extraCommandDirs(workspaceRoot) {
		for _, e := range loadCommandFilesFromDir(dir) {
			if _, ok := seenCmd[e.Slug]; ok {
				continue
			}
			seenCmd[e.Slug] = struct{}{}
			idx.Commands = append(idx.Commands, e)
		}
	}

	seenSkill := make(map[string]struct{}, len(idx.Skills))
	for _, s := range idx.Skills {
		seenSkill[s.Slug] = struct{}{}
	}
	for _, dir := range extraSkillDirs(workspaceRoot) {
		for _, e := range loadSkillFilesFromDir(dir) {
			if _, ok := seenSkill[e.Slug]; ok {
				continue
			}
			seenSkill[e.Slug] = struct{}{}
			idx.Skills = append(idx.Skills, e)
		}
	}

	return idx, nil
}

func extraSkillDirs(workspaceRoot string) []string {
	ws := filepath.Clean(workspaceRoot)
	dirs := []string{
		filepath.Join(ws, ".claude", "skills"),
		filepath.Join(ws, ".cursor", "skills"),
		filepath.Join(ws, ".codex", "skills"),
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dirs
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}

	dirs = append(dirs,
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".cursor", "skills"),
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(xdg, "opencode", "skills"),
	)
	return dirs
}

type skillFrontmatter struct {
	Description string `yaml:"description"`
}

func loadSkillFilesFromDir(dir string) []Entry {
	entries := make([]Entry, 0)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return entries
	}
	// Skills are organized as dir/skill-name/SKILL.md
	subdirs, err := os.ReadDir(dir)
	if err != nil {
		return entries
	}
	for _, sub := range subdirs {
		if !sub.IsDir() {
			continue
		}
		name := sub.Name()
		if name == "ROUTER.md" || name == "TEMPLATE.md" {
			continue
		}
		skillFile := filepath.Join(dir, name, "SKILL.md")
		e, ok := parseSkillMarkdownFile(skillFile, name)
		if !ok {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

func parseSkillMarkdownFile(path, slug string) (Entry, bool) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Entry{}, false
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(content, "\n")

	desc := ""
	// Try to extract description from YAML frontmatter if present.
	if len(lines) >= 3 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end > 1 {
			front := strings.Join(lines[1:end], "\n")
			var fm skillFrontmatter
			if yaml.Unmarshal([]byte(front), &fm) == nil && fm.Description != "" {
				desc = fm.Description
			}
		}
	}
	// Fallback: first non-empty non-heading line after frontmatter.
	if desc == "" {
		start := 0
		if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
			for i := 1; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "---" {
					start = i + 1
					break
				}
			}
		}
		for i := start; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			desc = line
			if len(desc) > 120 {
				desc = desc[:120]
			}
			break
		}
	}
	if desc == "" {
		desc = slug + " skill"
	}

	return Entry{
		Family:      FamilySkill,
		Slug:        slug,
		Description: desc,
		Insert:      "@" + slug,
	}, true
}

func extraCommandDirs(workspaceRoot string) []string {
	// Always include consumer workspace command directories.
	dirs := []string{
		filepath.Join(filepath.Clean(workspaceRoot), ".cursor", "commands"),
		filepath.Join(filepath.Clean(workspaceRoot), ".claude", "commands"),
	}

	// Global install paths come from scripts/install-global.{sh,ps1}.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dirs
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}

	dirs = append(dirs,
		filepath.Join(home, ".cursor", "commands"),
		filepath.Join(home, ".claude", "commands"),
		filepath.Join(xdg, "opencode", "commands"),
	)
	return dirs
}

type commandFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func loadCommandFilesFromDir(dir string) []Entry {
	entries := make([]Entry, 0)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return entries
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return entries
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if name == "ROUTER.md" || name == "TEMPLATE.md" {
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		full := filepath.Join(dir, name)
		e, ok := parseCommandMarkdownFile(full)
		if !ok {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

func parseCommandMarkdownFile(path string) (Entry, bool) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Entry{}, false
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return Entry{}, false
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end <= 1 {
		return Entry{}, false
	}

	front := strings.Join(lines[1:end], "\n")
	var fm commandFrontmatter
	if err := yaml.Unmarshal([]byte(front), &fm); err != nil {
		return Entry{}, false
	}

	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if strings.TrimSpace(fm.Name) != "" {
		slug = strings.TrimSpace(fm.Name)
	}

	desc := strings.TrimSpace(fm.Description)
	if desc == "" {
		desc = "Run the vibe-agent " + slug + " command"
	}

	return Entry{
		Family:      FamilyCommand,
		Slug:        slug,
		Description: desc,
		Insert:      "/" + slug,
	}, true
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
