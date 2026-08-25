package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/sandbox"
)

const sandboxUsage = `vibe-agent sandbox - workspace-opted runner drivers

Usage:
  vibe-agent sandbox init [--workspace <dir>]
  vibe-agent sandbox up   --slug <slug> --use-case <name> [--runner <name>]
  vibe-agent sandbox exec --slug <slug> --use-case <name> [--runner <name>] -- <command>...
  vibe-agent sandbox down --slug <slug> --use-case <name>

Drivers and use-case routing live in .agent-state/sandbox.yaml.
STATUS.md is written under .agent-state/runs/<date>/<slug>/<version>/sandbox/<use-case>/.
Embedded container runtimes stay declined; this command only orchestrates external drivers.
`

func sandboxCommand(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, sandboxUsage)
		return fmt.Errorf("sandbox needs a subcommand")
	}
	switch args[0] {
	case "init":
		return sandboxInitCommand(args[1:])
	case "up":
		return sandboxUpCommand(args[1:])
	case "exec":
		return sandboxExecCommand(args[1:])
	case "down":
		return sandboxDownCommand(args[1:])
	case "help", "-h", "--help":
		fmt.Print(sandboxUsage)
		return nil
	default:
		return fmt.Errorf("unknown sandbox subcommand %q", args[0])
	}
}

func sandboxInitCommand(args []string) error {
	flags := newFlagSet("sandbox init")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	path, err := sandbox.WriteExample(workspaceRoot)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", path)
	fmt.Println("edit runners and useCases, then: vibe-agent sandbox up --slug <slug> --use-case <name>")
	return nil
}

func sandboxUpCommand(args []string) error {
	flags := newFlagSet("sandbox up")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "run slug")
	useCase := flags.String("use-case", "", "use case name")
	runner := flags.String("runner", "", "optional runner override")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" || *useCase == "" {
		return fmt.Errorf("sandbox up needs --slug and --use-case")
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	if err := sandbox.Up(context.Background(), workspaceRoot, *slug, *useCase, *runner); err != nil {
		return err
	}
	path, err := sandbox.StatusPath(workspaceRoot, *slug, *useCase)
	if err != nil {
		return err
	}
	fmt.Printf("sandbox up: %s\n", path)
	return nil
}

func sandboxDownCommand(args []string) error {
	flags := newFlagSet("sandbox down")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "run slug")
	useCase := flags.String("use-case", "", "use case name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *slug == "" || *useCase == "" {
		return fmt.Errorf("sandbox down needs --slug and --use-case")
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	if err := sandbox.Down(workspaceRoot, *slug, *useCase); err != nil {
		return err
	}
	path, err := sandbox.StatusPath(workspaceRoot, *slug, *useCase)
	if err != nil {
		return err
	}
	fmt.Printf("sandbox down: %s\n", path)
	return nil
}

func sandboxExecCommand(args []string) error {
	flags := newFlagSet("sandbox exec")
	paths := addRootFlags(flags)
	slug := flags.String("slug", "", "run slug")
	useCase := flags.String("use-case", "", "use case name")
	runner := flags.String("runner", "", "optional runner override")
	timeoutSec := flags.Int("timeout", 0, "timeout in seconds (0 = default)")

	dash := indexDoubleDash(args)
	flagArgs := args
	cmdArgs := []string(nil)
	if dash >= 0 {
		flagArgs = args[:dash]
		cmdArgs = args[dash+1:]
	}
	if err := flags.Parse(flagArgs); err != nil {
		return err
	}
	if *slug == "" || *useCase == "" {
		return fmt.Errorf("sandbox exec needs --slug and --use-case")
	}
	if len(cmdArgs) == 0 {
		return fmt.Errorf("sandbox exec needs -- <command>")
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	timeout := time.Duration(0)
	if *timeoutSec > 0 {
		timeout = time.Duration(*timeoutSec) * time.Second
	}
	result := sandbox.Exec(context.Background(), sandbox.ExecRequest{
		WorkspaceRoot: workspaceRoot,
		Slug:          *slug,
		UseCase:       *useCase,
		Runner:        *runner,
		Command:       cmdArgs[0],
		Args:          cmdArgs[1:],
		Timeout:       timeout,
	})
	if len(result.Output) > 0 {
		if _, writeErr := os.Stdout.Write(result.Output); writeErr != nil {
			return writeErr
		}
		if !strings.HasSuffix(string(result.Output), "\n") {
			fmt.Println()
		}
	}
	if result.NeverRan {
		if result.Err != nil {
			return result.Err
		}
		return fmt.Errorf("sandbox exec could not start")
	}
	if result.TimedOut {
		return fmt.Errorf("sandbox exec timed out")
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("sandbox exec exited %d", result.ExitCode)
	}
	return nil
}

func indexDoubleDash(args []string) int {
	for i, a := range args {
		if a == "--" {
			return i
		}
	}
	return -1
}
