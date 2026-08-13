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
    run.go                run start, run status, run flag
    verify.go             run a verifier node's check and record what it found
    checkpoint.go         record evidence and advance the graph
    graph.go              graph validate
    mcp.go                mcp serve
    hook.go               lifecycle hooks for Claude and Cursor
    memory.go             list, confirm, forget. The human side of the store
    map.go                repository structure, cached by content hash
    doctor.go             workspace health checks
  internal/
    graph/                workflow graph model, loader, static validation
    checkplan/            vibe-checks.yaml: what command produces which check
    loop/                 the runner: transitions, budget, blocker stop rule
    checkpoint/           the one write path: apply evidence, once
      verify.go           the only producer of runtime-origin evidence
    verifier/             the things that produce evidence
      command.go          a subprocess exit code
      files.go            paths exist and are non-empty
      git.go              repository state, observed never changed
      screen.go           an app rendered: no crash, expected content, not blank
      device.go           adb and simctl, the only shell this verifier needs
    memory/               SQLite store, FTS search, write policy, promotion
    repomap/              declarations per file, cached by content hash
    mcp/                  stdio server, seven tools
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
make install    # into ~/.local/bin, the same place the installers use
make release    # all six targets plus SHA256SUMS
```

### Changing this module means reinstalling it

**Hooks call `vibe-agent` by name, so they run whatever is on `PATH` — not the source you just edited.** A passing `go test ./...` says nothing about what a session will execute.

Run `make install` after any change here, and `vibe-agent doctor` to confirm it took. `make install` writes to `$VIBE_INSTALL_DIR`, defaulting to `~/.local/bin` — the same location [`scripts/install-runtime.sh`](../scripts/install-runtime.sh) and [`.ps1`](../scripts/install-runtime.ps1) use, so there is one copy rather than one per installation route. It then reports when `PATH` resolves `vibe-agent` to a different build, because a shadowed install is invisible and its symptom is a hook behaving like a version you replaced.

Three failures this closes, all of which happened:

- A binary predating `post-tool-use` kept answering the other five hooks and refused that one. It read as a broken hook rather than an out-of-date install, and nothing compared the two: `make install` writes without checking, and the version string was `dev` on both sides so a version comparison proved nothing. A local build now stamps its commit.
- The three installation routes wrote to three different directories: `GOPATH/bin` here, `~/.local/bin` from the shell installer, and a third under `%LOCALAPPDATA%` from the PowerShell one. A machine ended up holding several binaries at different versions, and `PATH` order decided which the hooks called — so a `make install` could be silently ineffective because a copy installed elsewhere still won. All three now share one location, and all three report when `PATH` resolves `vibe-agent` to a different build.
- On Windows, `install` wrote an extensionless file. Git Bash can execute it; nothing else can resolve it by name. Worse, once the `.exe` existed too, a POSIX shell still picked the older extensionless one, so `vibe-agent` meant a different build depending on who asked. `make install` now appends `GOEXE`, and `doctor` reports the shadow if an old file is still there.

`doctor` reads the events `.claude/settings.json` and `.cursor/hooks.json` register, then asks the binary on `PATH` which ones it handles. It asks that binary rather than reporting its own list, because the process running `doctor` is not necessarily the one answering hooks — and that difference is the whole failure.

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

## Provenance is claimed, or it is produced

`--source exit_code` says what *kind* of evidence a check claims to be. It does not say a process ran. Anyone who could type the flag could walk a verifier node without verifying anything, and `exit_code` on `true` is a true statement.

Two things close that, and both live outside the model's reach:

1. **A verifier node's check can only be written by a verifier.** `checkpoint` refuses it and names the command that would work. The authority to write runtime-origin evidence is an unexported field on an unexported type in `internal/checkpoint`, obtainable only by calling `Verify`, which sets it after a verifier has returned. No other package can grant itself that, and the compiler is what enforces it.
2. **The command is not the caller's to choose either.** `verify` runs what [`vibe-checks.yaml`](../vibe-checks.yaml) declares for that check. Swapping a real suite for something weaker is a diff on a tracked file rather than an argument nobody reviews.

```sh
vibe-agent verify --slug my-feature --dry-run   # what would run, and from where
vibe-agent verify --slug my-feature             # run it and record the result
```

There is no `--passed` on `verify`. A failing check exits 0: the run recorded the failure and routed on it, which is the loop working, and a non-zero exit would make a host treat that as a broken tool call.

One escape hatch, deliberately visible. A check the plan declares `verifier: human` has no runtime verifier, so a person records it with `checkpoint --source human_event`. It costs a tracked diff to grant, and a plan full of them is a fact about the repo worth seeing rather than a setting to forget.

`doctor` reports any verifier node in any graph whose check the plan does not declare, because a run that discovers this mid-delivery discovers it at the expensive time.

## Proving an app rendered

An exit code cannot answer "did the user see the right thing". `flutter test integration_test` exits 0 whether the app painted a list or a white rectangle, so a repo whose mobile e2e check is only a command has a gate that a broken app walks through.

The `screen` verifier asks the device three separate questions:

| Signal | How | Catches |
|--------|-----|---------|
| Nothing crashed | `adb logcat -b crash -d`, cleared before launch | `FATAL EXCEPTION`, ANR, native tombstones |
| The expected content is on screen | `adb shell uiautomator dump`, matched against `expectText` and `expectResourceIds` | wrong data, an empty list, a permanent spinner |
| The frame is not blank | `adb exec-out screencap -p`, quantised colour histogram | a white or black screen with no crash to show for it |

They are independent because each has a blind spot the others do not. A crash-free app can show nothing; a busy screen can show the wrong numbers; a hierarchy can list nodes that never painted.

`forbidText` covers the case none of the three would catch on its own: a React Native redbox or a Flutter error widget is not an OS-level crash, so the crash buffer stays clean and the screen is busy. Naming the framework's own error text is what fails it.

Two limits worth knowing before trusting a pass:

- **Flutter renders to one canvas**, and that canvas is not accessible unless the app enables semantics. The dump then holds a single node, so a Flutter check has to assert content from inside the app instead. The Android adapter reports this rather than returning an empty tree, because empty would read as "the content is not there" when the truth is "nothing was measured".
- **iOS has no `uiautomator` equivalent.** `simctl` gives crash reports and screenshots; content on iOS comes from an XCUITest or a framework-level integration test.

The blank-frame test is deliberately conservative. The first version counted pixels near the frame's average colour, following US patent 7536078, and it failed on the commonest real case: a spinner on white drags the average away from white, leaving nothing near it and reporting 0% for the emptiest screen in the set. It now counts how many quantised colours the frame occupies at all, and fires only when there are almost none. Anti-aliased text alone puts a working screen well past that, which is the point: a verifier that failed real screens would be switched off, and that costs more than the cases it would catch.

## A skipped check is not a passed check

Three things used to be true at once, and together they meant a verification step could vanish while the run still reported green:

- `run.Flags` was read by the runner and written by nothing, so every `flag`-sourced guard was permanently false. The `research` node was unreachable for that reason.
- `skipWhen` was parsed and validated and never evaluated, so a declared skip condition did nothing.
- `evaluate` answered every `check`-sourced guard with `passed || skipped`, while the delivery graph documented `e2e_ok` as the only guard where the two were treated alike. They were treated alike everywhere.

All three are closed:

| Was | Is |
|-----|-----|
| Flags unwritable | `run flag --set <guard>`, only at a human gate, only for a `flag`-sourced guard, recorded as a `flag_set` event |
| `skipWhen` inert | Honored by `Runner.SkipReason`; a skip produces a real `skipped` check naming the guard |
| Every gate accepted a skip | Only a guard with `acceptsSkipped: true` does, and the validator rejects it on any source but `check` |

The delivery graph carries no `skipWhen` as a result. A flag absent from run state reads as false, so `skipWhen: "!e2e_required"` on the e2e node would skip it on every fresh run, and `e2e_ok` would pass it. What decides whether e2e runs is instead whether `vibe-checks.yaml` declares an `e2e` check: a workspace with no device or browser surface says so in a tracked file, and deleting that entry is a reviewable diff. `TestTheShippedGraphNeverSkipsAVerifierByDefault` fails if anyone adds a `skipWhen` back without deciding, per node, whether skipping by default is safe there.

## What is deterministic, and what is not

An MCP tool call happens because the model chose to make it. A hook happens because the host fired it. Both surfaces exist here; only one of them is a guarantee, and the split matters when reading `internal/harness`:

| Hook | Behavior | Refuses? |
|------|----------|----------|
| `session-start` | Injects rules, active runs, and stored memory. Steers a lone running goal through `initialUserMessage` | no |
| `user-prompt-submit` | Injects the current node and the memories matching the prompt, on every prompt | no |
| `pre-tool-use` | Refuses a push to a protected branch, `gh pr merge`, and any write to a run's manifest or event log | **yes** |
| `post-tool-use` | Appends a `tool_use` entry to the run's journal. Success only; a host fires this or the failure event, never both | no |
| `post-tool-use-failure` | The same entry marked `failed`, plus a confirmed memory citing what the host printed | no |
| `stop`, `subagent-stop` | Refuses to end the turn while a run sits mid-graph with nothing recorded | **yes** |

Three constraints hold this together:

- **`stop_hook_active` is the loop guard.** A blocking Stop hook that ignores it blocks its own continuation forever. `stop` blocks at most once per turn, and never for a run awaiting a human or one past `MaxBlockerAttempts`, because neither can be moved by another model turn.
- **Reads never create state.** `recall` opens the memory database only if it already exists, so a hook firing in an unrelated workspace leaves nothing behind. Only the `post-tool-use-failure` write path creates it.
- **An outcome is read, never inferred.** The host decides that a call failed, by firing `post-tool-use-failure` or by reporting a non-zero exit code, and either is an observation. Claude Code supplies no exit code in any field, so its memories say a command "fails" where Cursor's say it "exits 2". Quoting the host's own error text keeps the number legible without this package claiming to have parsed one; searching a result string for the word "error" would put a guess where this design accepts only evidence.
- **Both halves of an outcome are wired, or neither.** `doctor` fails a config registering `post-tool-use` alone. That config does not record less, it records the wrong half: every failure, which is what the journal exists for, is dropped in silence.

## Who confirms a memory

Retrieval returns confirmed memories only, so a store where nothing can reach that status is a store that is written and never read. Two paths reach it, and neither is the model:

```sh
vibe-agent memory list                  # what is stored, and at which status
vibe-agent memory confirm --id <id>     # a person vouches: source human_event
vibe-agent memory forget  --id <id>     # retract one that turned out wrong
```

The other path is `post-tool-use-failure`, which confirms from the host's own report that the call failed, citing the event-log entry it just wrote. Those memories carry an expiry of `FailureMemoryLife`, because "this command fails" is true about a moment and would otherwise become the stale memory this design keeps warning about.

This path was dead for the whole of the runtime's first life, and the shape of the failure is worth keeping: the journal read `tool_response` for an exit code, Claude Code sends none in any field, and every failing call went to `PostToolUseFailure`, an event no config registered and no build implemented. Unit tests passed against an invented payload, `doctor` reported the database opening, and `memory.db` stayed empty in every workspace. A green suite over a fixture nobody captured from the real host is the failure mode this whole package is otherwise built to prevent.

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

## Orienting without reading

`vibe-agent map` prints what a workspace declares and where. On this repository:
545 files, roughly 743,000 tokens to read them all, and a complete map of them
for about 5,000. The default budget of 2,000 buys the 173 files the rest of the
repository refers to most.

```sh
vibe-agent map                    # ranked, within the default budget
vibe-agent map --budget 8000      # the whole repository, if it fits
vibe-agent map --json             # the full index, ordered by path
vibe-agent map --refresh          # discard the cache and re-read everything
```

Four decisions are worth stating, because each had a cheaper option that was
worse:

- **Nothing here is a summary.** Every entry is a declaration at a line a reader
  can open. The moment a map paraphrases code it becomes model output claiming
  the authority of an index, and `internal/memory` refuses that everywhere else
  for the same reason.
- **The cache key is the content hash, not mtime.** A checkout moves mtime
  without changing code, and a restored file changes code without moving it. The
  cost is reading the file to hash it; the benefit is that the map cannot
  describe code that is no longer there. `cacheVersion` covers the other
  direction: when extraction changes, every row is stale while every hash still
  matches, so bumping it discards the index whole.
- **Regexes, not a parser.** A parser per language means tree-sitter, which means
  cgo or a vendored grammar per language, in a module whose entire dependency set
  is a pure-Go SQLite driver and a YAML reader. The honest cost: these find
  declarations, not call graphs.
- **Ranking is a reference count, not PageRank.** The
  [Aider repo map](https://aider.chat/2023/10/22/repomap.html) runs PageRank over
  a symbol graph; this counts how many other files mention each declaration. A
  name more than one file declares is dropped rather than weighted, because every
  script defines `main` and a mention of it attributes to nothing. Enough to
  decide what survives a budget, and explainable without eigenvectors.

Test files appear by name with their cases omitted. That a package has tests is
orientation; thirty function names each restating one assertion is most of a
budget spent on the part of a repository nobody navigates by.

## Contract with the schemas

[`schemas/run-state.schema.json`](../schemas/run-state.schema.json) is the contract; this module is one implementation of it. Two tests hold them together:

- `TestFreshRunMatchesGolden` pins what `Save` emits.
- `scripts/check-schemas.py` validates that golden file against the schema.

Without both, the writer and the contract drift and nothing notices until a run fails to load.

[`schemas/check-plan.schema.json`](../schemas/check-plan.schema.json) is held the same way: `internal/checkplan` is one implementation, and `check-schemas.py` validates this repo's real [`vibe-checks.yaml`](../vibe-checks.yaml) against the schema. A plan the Go loader accepts and the schema rejects is a drift the first stalled run would find otherwise.
