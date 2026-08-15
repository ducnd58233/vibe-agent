package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
)

// guardsCommand exposes the rules the guards run, and scaffolds a place to
// change them.
//
// Both halves exist for the same reason: the rules travel inside the binary, so
// without a way to list them a repository cannot see what it is overriding, and
// without a scaffold it has to learn the schema from source.
func guardsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("guards needs a subcommand: list or init")
	}
	switch args[0] {
	case "list":
		return guardsListCommand(args[1:])
	case "init":
		return guardsInitCommand(args[1:])
	default:
		return fmt.Errorf("unknown guards subcommand %q; try `vibe-agent guards list`", args[0])
	}
}

func guardsListCommand(args []string) error {
	flags := newFlagSet("guards list")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	summaries, err := harness.Guards(workspaceRoot)
	if err != nil {
		return err
	}

	fmt.Printf("guards for %s\n", workspaceRoot)
	for _, guard := range summaries {
		state := ""
		if guard.Disabled {
			state = "  (disabled)"
		}
		fmt.Printf("\n  %s%s\n", guard.Name, state)
		fmt.Printf("    reads   %s\n", guard.Applies)
		fmt.Printf("    checks  %s\n", strings.Join(guard.Checks, ", "))
	}
	fmt.Printf("\nChange these in %s. Start one with `vibe-agent guards init`.\n", harness.ConsumerGuardPlan)
	return nil
}

func guardsInitCommand(args []string) error {
	flags := newFlagSet("guards init")
	paths := addRootFlags(flags)
	force := flags.Bool("force", false, "overwrite an existing plan")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	path, err := harness.InitGuardPlan(workspaceRoot, *force)
	if errors.Is(err, harness.ErrGuardPlanExists) {
		return fmt.Errorf("%s already exists; edit it, or pass --force to replace it", path)
	}
	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", path)
	fmt.Println("Everything in it is commented out, so nothing changed yet.")
	fmt.Println("Run `vibe-agent guards list` to see what is running now.")
	return nil
}
