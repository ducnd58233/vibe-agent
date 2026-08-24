// Package skillsinstall builds argv for `npx skills add` and reports host-only
// Agent Skills frontmatter that may need stripping before other hosts load it.
package skillsinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultAgents are the four hosts vibe-agent targets. Names match the
// vercel-labs/skills CLI agent identifiers used in research.
var DefaultAgents = []string{
	"claude-code",
	"codex",
	"cursor",
	"opencode",
}

// hostOnlyFrontmatterKeys are Agent Skills YAML keys that are host dialect,
// not part of the shared agentskills.io body contract. Seeing one means the
// skill may still load elsewhere after a strip, not that install failed.
var hostOnlyFrontmatterKeys = map[string]string{
	"disable-model-invocation": "Claude Code / plugin harness: hide from model auto-load",
	"context":                  "Claude Code: often context: fork for subagent skills",
	"allowed-tools":            "Claude Code tool allow-list dialect",
	"user-invocable":           "Claude Code invocation gate",
	"hooks":                    "Host hook wiring; not portable Agent Skills content",
	"mcpServers":               "Embedded MCP; vibe-agent never auto-registers this",
	"mcp":                      "Embedded MCP alias; never auto-registers",
}

// AddOptions configure a forwarded `npx skills add` invocation.
type AddOptions struct {
	// Agents overrides DefaultAgents when non-empty. Each value becomes one -a.
	Agents []string
	// Global asks the skills CLI to install into user-level roots (-g).
	Global bool
	// Skill selects one skill name from a multi-skill source (--skill).
	Skill string
	// Yes passes -y so the CLI does not prompt.
	Yes bool
	// List lists skills in the source without installing (--list).
	List bool
}

// AddArgv returns the argv after the npx executable: ["skills", "add", ...].
// AddArgv returns the argv after the npx executable.
// When Yes is set the first token is --yes so npx itself does not prompt.
func AddArgv(source string, opts AddOptions) ([]string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("skills add needs a source (owner/repo or URL)")
	}
	if strings.HasPrefix(source, "-") {
		return nil, fmt.Errorf("skills add source must not start with -")
	}
	var argv []string
	if opts.Yes {
		argv = append(argv, "--yes")
	}
	argv = append(argv, "skills", "add", source)
	agents := opts.Agents
	if len(agents) == 0 && !opts.List {
		agents = append([]string(nil), DefaultAgents...)
	}
	for _, agent := range agents {
		agent = strings.TrimSpace(agent)
		if agent == "" {
			continue
		}
		if agent == "*" || strings.EqualFold(agent, "all") {
			return nil, fmt.Errorf("skills add refuses agent %q; name hosts explicitly", agent)
		}
		argv = append(argv, "-a", agent)
	}
	if opts.Global {
		argv = append(argv, "-g")
	}
	if opts.Yes {
		argv = append(argv, "-y")
	}
	if opts.List {
		argv = append(argv, "--list")
	}
	if skill := strings.TrimSpace(opts.Skill); skill != "" {
		argv = append(argv, "--skill", skill)
	}
	for _, token := range argv {
		if token == "mcp" || token == "hooks" || token == "--all" {
			return nil, fmt.Errorf("skills add refuses token %q (MCP and hooks are out of scope)", token)
		}
	}
	return argv, nil
}

// Finding is one host-only frontmatter key on a skill.
type Finding struct {
	Key     string
	Value   string
	Note    string
	SkillMD string
}

// Report is the convert-report for one skill directory (or a tree of them).
type Report struct {
	Root     string
	Findings []Finding
}

// ConvertReport scans skillDir for SKILL.md files and reports host-only keys.
// It never rewrites files.
func ConvertReport(skillDir string) (Report, error) {
	root, err := filepath.Abs(skillDir)
	if err != nil {
		return Report{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return Report{}, err
	}
	report := Report{Root: root}
	if !info.IsDir() {
		return report, fmt.Errorf("convert-report needs a directory, got %s", root)
	}

	skillPath := filepath.Join(root, "SKILL.md")
	if _, err := os.Stat(skillPath); err == nil {
		findings, err := scanSkillFile(skillPath)
		if err != nil {
			return report, err
		}
		report.Findings = append(report.Findings, findings...)
		sortFindings(report.Findings)
		return report, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return report, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		nested := filepath.Join(root, entry.Name(), "SKILL.md")
		if _, err := os.Stat(nested); err != nil {
			continue
		}
		findings, err := scanSkillFile(nested)
		if err != nil {
			return report, err
		}
		report.Findings = append(report.Findings, findings...)
	}
	sortFindings(report.Findings)
	return report, nil
}

func sortFindings(findings []Finding) {
	slices.SortFunc(findings, func(a, b Finding) int {
		if c := strings.Compare(a.SkillMD, b.SkillMD); c != 0 {
			return c
		}
		return strings.Compare(a.Key, b.Key)
	})
}

func scanSkillFile(path string) ([]Finding, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	fm, ok := extractFrontmatter(string(raw))
	if !ok {
		return nil, nil
	}
	keys, err := frontmatterKeyValues(fm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var findings []Finding
	for key, value := range keys {
		note, hostOnly := hostOnlyFrontmatterKeys[key]
		if !hostOnly {
			continue
		}
		findings = append(findings, Finding{
			Key:     key,
			Value:   value,
			Note:    note,
			SkillMD: path,
		})
	}
	return findings, nil
}

func extractFrontmatter(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return "", false
	}
	rest := trimmed[3:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	} else {
		return "", false
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func frontmatterKeyValues(fm string) (map[string]string, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(fm), &node); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return out, nil
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return out, nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]
		out[keyNode.Value] = scalarOrCompact(valNode)
	}
	return out, nil
}

func scalarOrCompact(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	b, err := yaml.Marshal(n)
	if err != nil {
		return n.Tag
	}
	return strings.TrimSpace(string(b))
}

// FormatReport prints a human-readable convert report.
func FormatReport(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "convert-report: %s\n", report.Root)
	if len(report.Findings) == 0 {
		b.WriteString("  no host-only frontmatter keys found\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  findings %d\n", len(report.Findings))
	for _, f := range report.Findings {
		rel := f.SkillMD
		if r, err := filepath.Rel(report.Root, f.SkillMD); err == nil {
			rel = r
		}
		fmt.Fprintf(&b, "  - %s (%s): %s = %q\n", rel, f.Note, f.Key, f.Value)
	}
	b.WriteString("  note: report only; files are unchanged. Strip host-only keys before sharing across hosts if a host rejects the skill.\n")
	return b.String()
}
