package main

import (
	"strings"
	"testing"
)

func TestUsageMentionsSkillsCommands(t *testing.T) {
	t.Parallel()
	if !strings.Contains(usage, "skills add") || !strings.Contains(usage, "skills convert-report") {
		t.Fatal("usage text missing skills commands")
	}
}

func TestSkillsHelp(t *testing.T) {
	t.Parallel()
	if err := skillsCommand([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
}
