package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/harness"
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
		return fmt.Errorf("memory needs a subcommand: list, propose, confirm, forget, review, promotions, history, link, sessions, gc")
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
	case "history":
		return memoryHistory(args[1:])
	case "link":
		return memoryLink(args[1:])
	case "sessions":
		return memorySessions(args[1:])
	case "gc":
		return memoryGC(args[1:])
	default:
		return fmt.Errorf("unknown memory subcommand %q; try list, propose, confirm, forget, review, promotions, history, link, sessions, or gc", args[0])
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
	client := flags.String("client", "", "host writing this: "+strings.Join(harness.ClientNames(), ", "))
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
	agent := flags.String("agent", "", "reviewing agent: host client, optionally /model, e.g. "+harness.ClientNames()[0]+"/<model>")
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

// memoryHistory prints a memory's ledger: every proposal, merge, confirmation,
// review, link, and closure, who did it, and when. The row itself only shows
// where the memory ended up.
func memoryHistory(args []string) error {
	flags := newFlagSet("memory history")
	paths := addRootFlags(flags)
	id := flags.String("id", "", "memory id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("memory history needs --id; run `vibe-agent memory list` to find one")
	}
	return withMemory(paths, func(store *memory.Store) error {
		ctx := context.Background()
		entries, err := store.History(ctx, *id)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No ledger entries. The memory predates the ledger.")
		}
		for _, entry := range entries {
			transition := ""
			if entry.ToStatus != "" {
				transition = fmt.Sprintf("  %s -> %s", orDash(entry.FromStatus), entry.ToStatus)
			}
			fmt.Printf("%s  %-10s%s  by=%s  %s\n", entry.At.Format(time.RFC3339), entry.Action,
				transition, orUnknown(entry.Actor), singleLine(entry.Detail))
		}
		links, err := store.Links(ctx, *id)
		if err != nil {
			return err
		}
		for _, link := range links {
			fmt.Printf("link: %s -[%s]-> %s\n", link.SrcID, link.Relation, link.DstID)
		}
		return nil
	})
}

// memoryLink adds a typed edge between two memories. Linking is collaboration
// metadata like review: it never changes a status, so it cannot confirm.
func memoryLink(args []string) error {
	flags := newFlagSet("memory link")
	paths := addRootFlags(flags)
	from := flags.String("from", "", "source memory id")
	to := flags.String("to", "", "destination memory id")
	relation := flags.String("relation", "relates_to", "supersedes, relates_to, derived_from, or contradicts")
	agent := flags.String("agent", "", "who is linking: host client, optionally /model")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *from == "" || *to == "" {
		return fmt.Errorf("memory link needs --from and --to; run `vibe-agent memory list` to find ids")
	}
	return withMemory(paths, func(store *memory.Store) error {
		link := memory.Link{SrcID: *from, DstID: *to, Relation: memory.Relation(*relation),
			CreatedBy: strings.TrimSpace(*agent)}
		if err := store.Link(context.Background(), link, time.Now().UTC()); err != nil {
			return err
		}
		fmt.Printf("%s -[%s]-> %s recorded. Statuses are unchanged.\n", *from, *relation, *to)
		return nil
	})
}

// memorySessions searches past conversations and tool calls by words. It is the
// episodic layer: what was said and run in earlier sessions, not what was
// distilled from them.
func memorySessions(args []string) error {
	flags := newFlagSet("memory sessions")
	paths := addRootFlags(flags)
	query := flags.String("query", "", "words to find in past sessions")
	limit := flags.Int("limit", memory.DefaultLimit, "most results to show")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*query) == "" {
		return fmt.Errorf("memory sessions needs --query")
	}
	return withMemory(paths, func(store *memory.Store) error {
		hits, err := store.SearchSessions(context.Background(), *query, *limit)
		if err != nil {
			return err
		}
		if len(hits) == 0 {
			fmt.Println("No past session matches.")
			return nil
		}
		for _, hit := range hits {
			fmt.Printf("%s  %s#%d  %-13s %s\n", hit.At.Format(time.RFC3339), hit.Scope, hit.Sequence,
				hit.Type, singleLine(hit.Snippet))
		}
		fmt.Printf("\n%s\n", memory.Disclaimer)
		return nil
	})
}

// memoryGC reclaims space from what can no longer be retrieved. By default it
// only removes memories whose own expiry passed over a week ago, which no query
// returns anyway; session history is kept unless a retention is asked for.
func memoryGC(args []string) error {
	flags := newFlagSet("memory gc")
	paths := addRootFlags(flags)
	expired := flags.Duration("expired-for", 7*24*time.Hour, "remove memories whose expiry passed at least this long ago (0 keeps them)")
	sessions := flags.Duration("sessions-older-than", 0, "remove session history older than this, e.g. 2160h for 90 days (0 keeps it)")
	dryRun := flags.Bool("dry-run", false, "count what would be removed without removing it")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return withMemory(paths, func(store *memory.Store) error {
		result, err := store.Prune(context.Background(), memory.PruneOptions{
			Now: time.Now().UTC(), ExpiredFor: *expired, SessionsOlderThan: *sessions, DryRun: *dryRun,
		})
		if err != nil {
			return err
		}
		verb := "removed"
		if *dryRun {
			verb = "would remove"
		}
		fmt.Printf("%s %d expired memor(ies), %d link(s), %d session event(s).\n",
			verb, result.Memories, result.Links, result.Sessions)
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
