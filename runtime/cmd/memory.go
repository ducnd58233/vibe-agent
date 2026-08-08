package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
)

// memoryCommand is the human side of the memory store.
//
// Two of the four statuses can only be reached by a person deciding something:
// confirmed, when a human vouches for a fact no verifier produced, and stale,
// when one turns out to be wrong. Without a way to reach them from a terminal,
// a memory the runtime got wrong would be permanent, and retrieval injects it
// into every session. This is that way.
func memoryCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("memory needs a subcommand: list, confirm, forget")
	}
	switch args[0] {
	case "list":
		return memoryListCommand(args[1:])
	case "confirm":
		return memorySetStatus(args[1:], "confirm")
	case "forget":
		return memorySetStatus(args[1:], "forget")
	default:
		return fmt.Errorf("unknown memory subcommand %q; try list, confirm, or forget", args[0])
	}
}

func memoryListCommand(args []string) error {
	flags := newFlagSet("memory list")
	paths := addRootFlags(flags)
	status := flags.String("status", "", "only this status: proposed, confirmed, stale, rejected")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	store, err := openExistingMemory(workspaceRoot)
	if err != nil {
		return err
	}
	if store == nil {
		fmt.Println("No memory database yet. One is created the first time something is stored.")
		return nil
	}
	defer store.Close()

	records, err := store.List(context.Background(), workspaceRoot)
	if err != nil {
		return err
	}

	shown := 0
	for _, record := range records {
		if *status != "" && string(record.Status) != *status {
			continue
		}
		shown++
		fmt.Printf("%s  %-9s %-10s used=%d%s\n", record.ID, record.Kind, record.Status,
			record.UsedCount, expiryNote(record.ExpiresAt))
		fmt.Printf("  %s\n", singleLine(record.Content))
		for _, item := range record.Evidence {
			fmt.Printf("    evidence: %s\n", singleLine(item))
		}
	}
	if shown == 0 {
		fmt.Println("No memories match.")
		return nil
	}

	// Retrieval returns confirmed memories only, so a store full of proposals
	// looks broken from the outside unless that is said out loud.
	fmt.Printf("\n%d shown. Only confirmed memories are retrieved into a session.\n", shown)
	return nil
}

func memorySetStatus(args []string, action string) error {
	flags := newFlagSet("memory " + action)
	paths := addRootFlags(flags)
	id := flags.String("id", "", "memory id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("memory %s needs --id; run `vibe-agent memory list` to find one", action)
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}

	store, err := openExistingMemory(workspaceRoot)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("no memory database at %s", memory.DBPath(workspaceRoot))
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	if action == "forget" {
		if err := store.SetStatus(ctx, *id, memory.StatusStale, now); err != nil {
			return err
		}
		fmt.Printf("%s is now stale and will not be retrieved.\n", *id)
		return nil
	}

	// SourceHumanStatement is the honest provenance here: a person at a terminal
	// vouched for it. Recording it as a command result would forge the evidence
	// this whole store exists to keep honest.
	record, err := store.Confirm(ctx, *id, memory.SourceHumanStatement, "human confirmation via vibe-agent memory confirm", now)
	if err != nil {
		return err
	}
	fmt.Printf("%s is confirmed and will be retrieved into future sessions.\n", record.ID)
	return nil
}

// openExistingMemory opens the store without creating one, so a command run in
// the wrong directory reports that rather than seeding an empty database there.
func openExistingMemory(workspaceRoot string) (*memory.Store, error) {
	path := memory.DBPath(workspaceRoot)
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	return memory.OpenAt(path)
}

func expiryNote(expires *time.Time) string {
	if expires == nil {
		return ""
	}
	return "  expires=" + expires.Format(memory.ExpiryLayout)
}

func singleLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
