package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
		return fmt.Errorf("memory needs a subcommand: list, propose, confirm, forget, review, promotions")
	}
	switch args[0] {
	case "list":
		return memoryListCommand(args[1:])
	case "propose":
		return memoryPropose(args[1:])
	case "promotions":
		return memoryPromotions(args[1:])
	case "confirm":
		return memorySetStatus(args[1:], "confirm")
	case "forget":
		return memorySetStatus(args[1:], "forget")
	case "review":
		return memoryReview(args[1:])
	default:
		return fmt.Errorf("unknown memory subcommand %q; try list, propose, confirm, forget, review, or promotions", args[0])
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

	store, exists, err := openExistingMemory(workspaceRoot)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Println("No memory database yet. One is created the first time something is stored.")
		return nil
	}
	defer func() { _ = store.Close() }()

	records, err := store.List(context.Background(), memory.WorkspaceKey(workspaceRoot))
	if err != nil {
		return err
	}

	shown := 0
	for _, record := range records {
		if *status != "" && string(record.Status) != *status {
			continue
		}
		shown++
		fmt.Printf("%s  %-9s %-10s used=%d%s%s\n", record.ID, record.Kind, record.Status,
			record.UsedCount, stamp("  expires=", record.ExpiresAt), stamp("  closed=", record.ValidTo))
		fmt.Printf("  %s\n", singleLine(record.Content))
		if record.CreatedBy != "" || len(record.ReviewedBy) > 0 {
			fmt.Printf("    by: %s  reviewed by: %s\n", orUnknown(record.CreatedBy), orNone(record.ReviewedBy))
		}
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
	return withMemory(paths, func(store *memory.Store) error {
		ctx := context.Background()
		now := time.Now().UTC()

		if action == "forget" {
			// Invalidate rather than SetStatus: closing the validity interval is
			// what records when the fact stopped being true, which is the part an
			// as-of query needs and a status flag cannot carry.
			if err := store.Invalidate(ctx, *id, now); err != nil {
				return err
			}
			fmt.Printf("%s is closed as of now and will not be retrieved.\n", *id)
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
	})
}

// withMemory runs one edit against the workspace's existing memory database.
// It never creates one, and a failed close is reported rather than dropped.
func withMemory(paths *rootFlags, edit func(*memory.Store) error) (err error) {
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	store, exists, err := openExistingMemory(workspaceRoot)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("no memory database at %s", memory.DBPath(workspaceRoot))
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	return edit(store)
}

func orUnknown(author string) string {
	if author == "" {
		return "unknown"
	}
	return author
}

func orNone(agents []string) string {
	if len(agents) == 0 {
		return "none"
	}
	return strings.Join(agents, ", ")
}

// memoryPropose is the remember step: it stores a lesson as a proposal through
// the same policy filter the hooks and MCP use. It can never confirm, and a
// rejection is an error so a script cannot mistake it for a stored memory.
func memoryPropose(args []string) (err error) {
	flags := newFlagSet("memory propose")
	paths := addRootFlags(flags)
	kind := flags.String("kind", "", "semantic, episodic, correction, or preference")
	content := flags.String("content", "", "the lesson, one claim")
	sourceType := flags.String("source-type", "", "command_result, file_content, ci_api, human_statement, or review_comment")
	sourceRef := flags.String("source-ref", "", "where the evidence came from (run event, log path)")
	client := flags.String("client", "", "host writing this: claude-code, cursor, codex, opencode, ...")
	model := flags.String("model", "", "model id, when known")
	confidence := flags.Float64("confidence", 0.7, "0 to 1")
	var evidence, tags multiFlag
	flags.Var(&evidence, "evidence", "a concrete observation; repeat for more")
	flags.Var(&tags, "tag", "a retrieval tag; repeat for more")
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	store, err := memory.Open(context.Background(), workspaceRoot)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, store.Close()) }()

	stored, decision, err := store.Propose(context.Background(), memory.Record{
		WorkspaceID: memory.WorkspaceKey(workspaceRoot),
		Kind:        memory.Kind(*kind),
		Content:     *content,
		Tags:        tags,
		Confidence:  *confidence,
		SourceType:  memory.SourceType(*sourceType),
		SourceRef:   *sourceRef,
		Evidence:    evidence,
		CreatedBy:   memory.Author(*client, *model),
	}, time.Now().UTC())
	if err != nil {
		return err
	}
	if decision.Verdict == memory.VerdictReject {
		return fmt.Errorf("memory rejected: %s", decision.Reason)
	}
	fmt.Printf("%s %s as %s. Confirmation needs a verifier result or a person (`memory confirm`).\n",
		stored.ID, decision.Verdict, stored.Status)
	return nil
}

// memoryPromotions is the improve step: it lists confirmed memories reused
// often enough to deserve a reviewed rule, and where that rule would go. It
// writes nothing; a person or a reviewed change moves the rule.
func memoryPromotions(args []string) (err error) {
	flags := newFlagSet("memory promotions")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	store, exists, err := openExistingMemory(workspaceRoot)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Println("No memory database yet, so nothing has been reused enough to promote.")
		return nil
	}
	defer func() { err = errors.Join(err, store.Close()) }()

	records, err := store.List(context.Background(), memory.WorkspaceKey(workspaceRoot))
	if err != nil {
		return err
	}
	promotions := memory.ProposePromotions(records)
	if len(promotions) == 0 {
		fmt.Printf("No confirmed memory has been reused %d times yet.\n", memory.PromotionThreshold)
		return nil
	}
	for _, promotion := range promotions {
		fmt.Printf("%s  used=%d  -> %s\n  %s\n  why: %s\n", promotion.Record.ID, promotion.Record.UsedCount,
			promotion.Target, singleLine(promotion.Record.Content), promotion.Reason)
	}
	fmt.Printf("\n%d promotion(s) proposed. Nothing was written; each needs a reviewed change.\n", len(promotions))
	return nil
}

// memoryReview records that an agent audited a memory. It does not confirm it:
// a review is collaboration metadata, and confirmation stays with a verifier
// result or a person (`memory confirm`).
func memoryReview(args []string) error {
	flags := newFlagSet("memory review")
	paths := addRootFlags(flags)
	id := flags.String("id", "", "memory id")
	agent := flags.String("agent", "", "reviewing agent: host client, optionally /model (claude, codex/gpt-5)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id == "" || strings.TrimSpace(*agent) == "" {
		return fmt.Errorf("memory review needs --id and --agent; run `vibe-agent memory list` to find an id")
	}
	return withMemory(paths, func(store *memory.Store) error {
		if err := store.AddReviewer(context.Background(), *id, *agent, time.Now().UTC()); err != nil {
			return err
		}
		fmt.Printf("%s: review by %s recorded. Its status is unchanged; confirming is separate.\n", *id, strings.TrimSpace(*agent))
		return nil
	})
}

// openExistingMemory opens the store without creating one, so a command run in
// the wrong directory reports that rather than seeding an empty database there.
func openExistingMemory(workspaceRoot string) (store *memory.Store, exists bool, err error) {
	path := memory.DBPath(workspaceRoot)
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("check for memory database: %w", statErr)
	}
	store, err = memory.OpenAt(context.Background(), path)
	return store, err == nil, err
}

func stamp(label string, value *time.Time) string {
	if value == nil {
		return ""
	}
	return label + value.Format(memory.ExpiryLayout)
}

func singleLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
