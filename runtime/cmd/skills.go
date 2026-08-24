package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/skillsinstall"
)

const skillsAddTimeout = 10 * time.Minute

// skillsCommand installs third-party Agent Skills via npx skills, or reports
// host-only frontmatter. It never writes MCP configs or host hooks.
func skillsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("skills needs a subcommand: add, convert-report")
	}
	switch args[0] {
	case "add":
		return skillsAddCommand(args[1:])
	case "convert-report":
		return skillsConvertReportCommand(args[1:])
	case "help", "-h", "--help":
		fmt.Print(skillsUsage)
		return nil
	default:
		return fmt.Errorf("unknown skills subcommand %q; try `vibe-agent skills --help`", args[0])
	}
}

const skillsUsage = `vibe-agent skills - third-party Agent Skills helpers

Usage:
  vibe-agent skills add <owner/repo|url> [-a <agent>]... [-g] [-y] [--skill <name>] [--list]
  vibe-agent skills convert-report <skill-dir>

"add" forwards to "npx skills add". With no -a flags it targets the four
vibe-agent hosts: claude-code, codex, cursor, opencode. It never writes MCP
or hooks. Pass DISABLE_TELEMETRY=1 into the child environment.

"convert-report" scans SKILL.md frontmatter for host-only keys
(disable-model-invocation, context, allowed-tools, and similar) and prints them. It
does not rewrite files.

Examples:
  vibe-agent skills add vercel-labs/agent-skills -g -y --skill writing-guidelines
  vibe-agent skills add some/repo -a cursor -a opencode -g -y
  vibe-agent skills convert-report ~/.agents/skills/writing-guidelines
`

func skillsAddCommand(args []string) error {
	flags := newFlagSet("skills add")
	global := flags.Bool("g", false, "install into user-level skill roots")
	yes := flags.Bool("y", false, "do not prompt")
	list := flags.Bool("list", false, "list skills in the source without installing")
	skill := flags.String("skill", "", "install only this skill from a multi-skill source")
	var agents multiFlag
	flags.Var(&agents, "a", "target agent (repeatable); default: claude-code,codex,cursor,opencode")
	if err := flags.Parse(args); err != nil {
		return err
	}
	rest := flags.Args()
	if len(rest) == 0 {
		return fmt.Errorf("skills add needs a source (owner/repo or URL)")
	}
	source := rest[0]
	if err := flags.Parse(rest[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("skills add takes one source, got extra: %s", strings.Join(flags.Args(), " "))
	}

	argv, err := skillsinstall.AddArgv(source, skillsinstall.AddOptions{
		Agents: []string(agents),
		Global: *global,
		Yes:    *yes,
		List:   *list,
		Skill:  *skill,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), skillsAddTimeout)
	defer cancel()

	cmd, err := safexec.CommandContext(ctx, "npx", argv...)
	if err != nil {
		return fmt.Errorf("skills add needs npx on PATH: %w", err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"DISABLE_TELEMETRY=1",
		"DO_NOT_TRACK=1",
	)
	fmt.Fprintf(os.Stderr, "vibe-agent: running npx %s\n", strings.Join(argv, " "))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npx skills add: %w", err)
	}
	return nil
}

func skillsConvertReportCommand(args []string) error {
	flags := newFlagSet("skills convert-report")
	if err := flags.Parse(args); err != nil {
		return err
	}
	rest := flags.Args()
	if len(rest) != 1 {
		return fmt.Errorf("skills convert-report needs one skill directory")
	}
	report, err := skillsinstall.ConvertReport(rest[0])
	if err != nil {
		return err
	}
	fmt.Print(skillsinstall.FormatReport(report))
	return nil
}

// multiFlag collects repeatable string flags.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
