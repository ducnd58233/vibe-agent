# vibe-agent runtime

Outer-loop control plane for this toolkit. Optional: every asset under [`.ai-agents/`](../.ai-agents) works with this binary absent.

## What it owns

The **outer** delivery loop: run state, graph transitions, verification evidence, and the gates between them.

It does not own the **inner** loop. Claude Code, Codex, Cursor, and opencode keep their own model and tool loops. This binary decides what happens between their turns. See [`loop-and-graph-engineering.md`](../.ai-agents/references/loop-and-graph-engineering.md).

## Do users need Go?

No. Contributors to this module need Go; users of the toolkit get a prebuilt binary. The module builds with `CGO_ENABLED=0`, so no C toolchain is needed either.

## Layout

```
runtime/
  Makefile                build, test, and release commands
  cmd/                    the CLI, one file per command
    main.go               entry point and the dispatch table
    common.go             shared path flags and output helpers
    run.go                run start, run status
    checkpoint.go         record evidence and advance the graph
    graph.go              graph validate
    mcp.go                mcp serve
    hook.go               lifecycle hooks for Claude and Cursor
    memory.go             list, confirm, forget. The human side of the store
    doctor.go             workspace health checks
  internal/
    graph/                workflow graph model, loader, static validation
    loop/                 the runner: transitions, budget, blocker stop rule
    checkpoint/           the one write path: apply evidence, once
    verifier/             command, files, git. The things that produce evidence
    memory/               SQLite store, FTS search, write policy, promotion
    mcp/                  stdio server, six tools
    harness/              hook adapters for Claude and Cursor
      gate.go             the refusals: protected pushes, merges, state writes
      recall.go           retrieval that does not wait to be asked
      journal.go          what a tool did, written down after it did it
    state/                run state and the append-only event log
      testdata/           golden manifest, validated by scripts/check-schemas.py
  e2e/                    drives the built binary against a fixture consumer repo
```

Three conventions worth knowing:

- **One file per command under `cmd/`.** `main.go` stays a dispatch table; command logic never accumulates there. `common.go` holds the `--workspace` and `--toolkit` pair every command shares, so adding a command does not mean copying flag plumbing.
- **The binary name comes from `-o`, not the directory.** `go build ./cmd` without `-o` would produce `cmd`; every build path here passes `-o vibe-agent`.
- **Test fixtures live in the `testdata/` of the package that uses them**, which is the Go convention. `internal/state/testdata/` belongs to the state package; nothing else reads it except the schema check, by path.

## Build and test

Use the Makefile. `make help` lists everything.

```sh
make check      # gofmt, vet, and every test including the e2e suite
make build      # dist/vibe-agent for this platform
make install    # onto $(go env GOPATH)/bin for local use
make release    # all six targets plus SHA256SUMS
```

Regenerate the golden manifest after an intentional shape change:

```sh
UPDATE_GOLDEN=1 go test ./internal/state -run TestFreshRunMatchesGolden
```

## Try it

```sh
vibe-agent run start --slug my-feature --goal "add webhook idempotency"
vibe-agent run status --slug my-feature
```

State lands in `tmp/my-feature/manifest.json` with the log at `tmp/my-feature/events.ndjson`, beside the human-readable `RECORD.md` described in [`goal-verification-records.md`](../.ai-agents/references/goal-verification-records.md). Both are gitignored.

## The rule this module enforces

A check is `passed` only when real evidence produced it. `CheckSource` has four values: `exit_code`, `file_assert`, `ci_api`, `human_event`. There is deliberately no value for model assertion, so no code path lets model output mark its own work complete.

`Load` applies the same rule as `SetCheck`, so a hand-edited manifest gets the same scrutiny as one this package wrote.

## What is deterministic, and what is not

An MCP tool call happens because the model chose to make it. A hook happens because the host fired it. Both surfaces exist here; only one of them is a guarantee, and the split matters when reading `internal/harness`:

| Hook | Behavior | Refuses? |
|------|----------|----------|
| `session-start` | Injects rules, active runs, and stored memory. Steers a lone running goal through `initialUserMessage` | no |
| `user-prompt-submit` | Injects the current node and the memories matching the prompt, on every prompt | no |
| `pre-tool-use` | Refuses a push to a protected branch, `gh pr merge`, and any write to a run's manifest or event log | **yes** |
| `post-tool-use` | Appends a `tool_use` entry to the run's journal, and proposes a memory when a command reported a non-zero exit | no |
| `stop`, `subagent-stop` | Refuses to end the turn while a run sits mid-graph with nothing recorded | **yes** |

Three constraints hold this together:

- **`stop_hook_active` is the loop guard.** A blocking Stop hook that ignores it blocks its own continuation forever. `stop` blocks at most once per turn, and never for a run awaiting a human or one past `MaxBlockerAttempts`, because neither can be moved by another model turn.
- **Reads never create state.** `recall` opens the memory database only if it already exists, so a hook firing in an unrelated workspace leaves nothing behind. Only the `post-tool-use` write path creates it.
- **An outcome is read, never inferred.** `post-tool-use` proposes a memory only when the host reported a structured exit code. Parsing a result string for the word "error" would put a guess where this design accepts only evidence.

## Who confirms a memory

Retrieval returns confirmed memories only, so a store where nothing can reach that status is a store that is written and never read. Two paths reach it, and neither is the model:

```sh
vibe-agent memory list                  # what is stored, and at which status
vibe-agent memory confirm --id <id>     # a person vouches: source human_event
vibe-agent memory forget  --id <id>     # retract one that turned out wrong
```

The other path is `post-tool-use`, which confirms from the exit code the host reported, citing the event-log entry it just wrote. Those memories carry an expiry of `FailureMemoryLife`, because "this command fails" is true about a moment and would otherwise become the stale memory this design keeps warning about.

`forget` closes the memory's validity interval rather than flipping a flag, so the store still knows when the fact stopped being true. It is one way: a retracted memory is re-earned with fresh evidence rather than toggled back on.
## Recording a checkpoint twice

`internal/checkpoint` owns the write path both the CLI and the MCP tool go through, and it records a given piece of evidence once.

Every transition event carries a key derived from what the checkpoint asserts: the check, its verdict, its source and ref, any result flags, any blocker. If the incoming evidence matches the key on the last transition, nothing advances and the caller is told so.

Two things are deliberately outside the key:

- **The clock**, so a retry a second later is recognised rather than looking new.
- **The current node**, because a retry arrives *after* the first attempt already moved the run. Keying on where the run is now would make the replay this exists to catch look like a different checkpoint.

An outcome that asserts nothing gets no key at all. A bare advance past an agent node has nothing to recognise a replay by, and treating two of them as the same event would stall a run walking through consecutive nodes.

## When a memory was true, and when we learned it

Memories carry a validity interval (`valid_from`, `valid_to`) separately from when the store learned about them (`created_at`, `updated_at`). This is the [bi-temporal model from Zep's Graphiti](https://arxiv.org/pdf/2501.13956), reduced to what this store needs.

Confirming a memory that supersedes another **closes** the old one at the new one's `valid_from` rather than deleting it. Retrieval stops returning it, so both sides of a contradiction never arrive together, while the record of having believed it survives:

```go
store.Search(ctx, memory.Query{WorkspaceID: id})                  // what is held now
store.Search(ctx, memory.Query{WorkspaceID: id, AsOf: lastMonth}) // what was held then
```

An as-of query includes stale memories by default. A fact that was true then is usually stale now, and excluding it would make every as-of query answer with the present.

## How retrieval ranks

With a query, two orderings are fused with [reciprocal rank fusion](https://dl.acm.org/doi/10.1145/1571941.1572114) at `k=60`: bm25 keyword relevance, and recency. Neither alone is right, and fusing avoids inventing a scale on which a bm25 score and a timestamp are comparable.

Ties are frequent and expected. Two results that swap places between the two rankings score identically, so the tiebreak is recency, then confidence: when two memories say almost the same thing, the usual reason is that one replaced the other.

Keyword search with metadata filters remains the deliberate first choice. Embeddings come only after this is measured as insufficient.

## Contract with the schemas

[`schemas/run-state.schema.json`](../schemas/run-state.schema.json) is the contract; this module is one implementation of it. Two tests hold them together:

- `TestFreshRunMatchesGolden` pins what `Save` emits.
- `scripts/check-schemas.py` validates that golden file against the schema.

Without both, the writer and the contract drift and nothing notices until a run fails to load.
