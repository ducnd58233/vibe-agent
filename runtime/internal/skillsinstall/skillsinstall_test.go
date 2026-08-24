package skillsinstall

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAddArgvDefaultsFourAgents(t *testing.T) {
	t.Parallel()
	argv, err := AddArgv("vercel-labs/agent-skills", AddOptions{Global: true, Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []string{"skills", "add", "vercel-labs/agent-skills"}
	if !slices.Equal(argv[:3], wantPrefix) {
		t.Fatalf("prefix = %v, want %v", argv[:3], wantPrefix)
	}
	for _, agent := range DefaultAgents {
		if !containsPair(argv, "-a", agent) {
			t.Fatalf("missing -a %s in %v", agent, argv)
		}
	}
	if !slices.Contains(argv, "-g") || !slices.Contains(argv, "-y") {
		t.Fatalf("expected -g and -y in %v", argv)
	}
}

func TestAddArgvAgentOverride(t *testing.T) {
	t.Parallel()
	argv, err := AddArgv("owner/repo", AddOptions{Agents: []string{"cursor"}, Skill: "writing-guidelines"})
	if err != nil {
		t.Fatal(err)
	}
	if containsPair(argv, "-a", "claude-code") {
		t.Fatalf("override should drop defaults: %v", argv)
	}
	if !containsPair(argv, "-a", "cursor") {
		t.Fatalf("missing cursor: %v", argv)
	}
	if !containsPair(argv, "--skill", "writing-guidelines") {
		t.Fatalf("missing --skill: %v", argv)
	}
}

func TestAddArgvRejectsEmptySource(t *testing.T) {
	t.Parallel()
	if _, err := AddArgv("  ", AddOptions{}); err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestConvertReportFindsHostOnlyKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skill := filepath.Join(dir, "demo")
	if err := os.Mkdir(skill, 0o750); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: demo\ndescription: test\ndisable-model-invocation: true\ncontext: fork\n---\n\n# Demo\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := ConvertReport(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range report.Findings {
		got[f.Key] = true
	}
	if !got["disable-model-invocation"] || !got["context"] {
		t.Fatalf("findings = %#v", report.Findings)
	}
	text := FormatReport(report)
	if !strings.Contains(text, "disable-model-invocation") {
		t.Fatalf("format missing key: %s", text)
	}
}

func TestConvertReportCleanSkill(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := "---\nname: clean\ndescription: portable\n---\n\n# Clean\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := ConvertReport(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", report.Findings)
	}
}

func containsPair(argv []string, flag, value string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == value {
			return true
		}
	}
	return false
}
