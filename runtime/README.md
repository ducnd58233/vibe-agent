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
  main.go                 CLI entry point
  Makefile                build, test, and release commands
  internal/
    graph/                workflow graph model, loader, static validation
    loop/                 the runner: transitions, budget, blocker stop rule
    verifier/             command, files, git. The things that produce evidence
    memory/               SQLite store, FTS search, write policy, promotion
    mcp/                  stdio server, six tools
    harness/              hook adapters for Claude and Cursor
    state/                run state and the append-only event log
      testdata/           golden manifest, validated by scripts/check-schemas.py
  e2e/                    drives the built binary against a fixture consumer repo
```

Two conventions worth knowing:

- **`main.go` sits at the module root, not under `cmd/`.** The Go docs call `cmd/` *"not strictly necessary in a repository that consists only of commands"*; this module has one binary and only `internal/` packages, none of which are importable from outside. Builds always pass `-o vibe-agent`, so the directory name never decides the binary name.
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

## Contract with the schemas

[`schemas/run-state.schema.json`](../schemas/run-state.schema.json) is the contract; this module is one implementation of it. Two tests hold them together:

- `TestFreshRunMatchesGolden` pins what `Save` emits.
- `scripts/check-schemas.py` validates that golden file against the schema.

Without both, the writer and the contract drift and nothing notices until a run fails to load.
