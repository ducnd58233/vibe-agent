package main

import (
	"fmt"

	"github.com/ducnd58233/vibe-agent/runtime/internal/autoconfig"
)

// The auto command surface. Only `init` exists so far: it writes the opt-in a
// workspace answers before auto mode may merge anything.
//
// The run itself lands with the graph edges that skip the approval gates. This
// half is separate on purpose, because the opt-in is the thing a person reads
// and it should not arrive in the same change as the machinery that reads it.

func autoCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("auto needs a subcommand; the only one is `init`")
	}
	if args[0] != "init" {
		return fmt.Errorf("unknown auto subcommand %q; the only one is `init`", args[0])
	}
	return autoInitCommand(args[1:])
}

// autoInitCommand writes the opt-in template and says what to do with it.
func autoInitCommand(args []string) error {
	flags := newFlagSet("auto init")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	path, err := autoconfig.Write(workspaceRoot)
	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", path)
	fmt.Println("  auto mode will not merge until someone sets merge: true in it")
	fmt.Println("  the file is gitignored with the rest of .agent-state, so the answer is per checkout")
	return nil
}
