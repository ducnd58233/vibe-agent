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
    run.go                goal, research, auto, run start, run status, run flag
    verify.go             run a verifier node's check and record what it found
    checkpoint.go         record evidence and advance the graph
    graph.go              graph validate
    mcp.go                mcp serve
    hook.go               lifecycle hooks for Claude and Cursor
    memory.go             list, confirm, forget. The human side of the store
    fetch.go              a URL or file as text, cached by source
    doctor.go             workspace health checks
    web.go                loopback web UI (127.0.0.1 only)
  web/                    embedded templates, static assets, view models
    render.go             embed + template funcs
    static/               tokens.css, shell.css, htmx.min.js, composer.js
    templates/            html/template pages
    view/                 event projection, pages, inspector, usage
  internal/
    shared/
      workspace/          directory names every module agrees on
      redact/             credential-shaped text replacement for logs and UI
      observability/      structured logging (console + JSON file)
      infra/
        database/         SQLite driver registration and open helper
        httpserver/
          middleware/     recover, access log, chain, request id
          response.go     JSON and error envelope helpers
          server.go       graceful shutdown via context
    web/
      app/                HTTP server composition root (bootstrap, routes)
      domain/             registry, workspace path rules
      infra/
        catalog/          ROUTER.md parsers for composer autocomplete
        persistence/      web.json and workspace registry writer
    run/                  one delivery run: where it sits, what was recorded
      domain/             Run, Check, Event, the provenance enum, and its rules
      infra/persistence/  the manifest and the append-only event log
    graph/                which node comes next, and on what evidence
      domain/             nodes, edges, guards, static validation
      infra/              reading a graph off disk
    memory/               what an earlier run learned
      domain/             Record, Kind, Status, and the write policy
      app/ports.go        Store, the repository callers depend on
      infra/persistence/  the SQLite table and its FTS search
    fetch/                a URL or file as the text an agent needs
      domain/             Document, Status, and how to classify one
      app/                ports, and the order the steps happen in
      infra/              the request, the extractor, the cache
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
    mcp/                  stdio server, seven tools
    harness/              hook adapters for Claude and Cursor
      gate.go             the refusals: protected pushes, merges, state writes
      recall.go           retrieval that does not wait to be asked
      journal.go          what a tool did, written down after it did it
  e2e/                    drives the built binary against a fixture consumer repo
```

Three conventions worth knowing:

- **One file per command under `cmd/`.** `main.go` stays a dispatch table; command logic never accumulates there. `common.go` holds the `--workspace` and `--toolkit` pair every command shares, so adding a command does not mean copying flag plumbing.
- **The binary name comes from `-o`, not the directory.** `go build ./cmd` without `-o` would produce `cmd`; every build path here passes `-o vibe-agent`.
- **Test fixtures live in the `testdata/` of the package that uses them**, which is the Go convention. `internal/run/infra/persistence/testdata/` belongs to the package that writes manifests; nothing else reads it except the schema check, by path.

## Build and test

Use the Makefile. `make help` lists everything.

```sh
make check      # gofmt, vet, and every test including the e2e suite
make build      # dist/vibe-agent for this platform
make install    # into ~/.local/bin, the same place the installers use
make release    # all six targets plus SHA256SUMS
```

### Published binaries

CI publishes two release channels on GitHub:

| Channel | Tag | When | Example version |
|---|---|---|---|
| Stable | `runtime/v*`, marked GitHub **Latest** | Someone tags `runtime/v0.1.0` | `v0.1.0` |
| Rolling | `runtime/latest`, prerelease | Every push to `main` that touches `runtime/` | `0.2.0-dev.159.2a82465` |

The rolling name has three parts:

1. **`0.2.0`** — conventional semver preview of the next stable cut (from commits since the last stable tag).
2. **`159`** — build counter: commits on `main` since that stable tag. It increments on every rolling publish even when the preview base stays `0.2.0`.
3. **`2a82465`** — the git commit the binary was built from.

[`scripts/install-runtime.sh`](../scripts/install-runtime.sh) resolves `latest` to the newest stable release first, then falls back to `runtime/latest`. Pass `v0.1.0` to pin a stable cut, or set channel `rolling` for the build from main.

Version derivation lives in [`scripts/runtime-version.sh`](../scripts/runtime-version.sh) and is checked by [`scripts/check-runtime-version.sh`](../scripts/check-runtime-version.sh).

### Changing this module means reinstalling it

**Hooks call `vibe-agent` by name, so they run whatever is on `PATH` — not the source you just edited.** A passing `go test ./...` says nothing about what a session will execute.

Run `make install` after any change here, and `vibe-agent doctor` to confirm it took. `make install` writes to `$VIBE_INSTALL_DIR`, defaulting to `~/.local/bin` — the same location [`scripts/install-runtime.sh`](../scripts/install-runtime.sh) and [`.ps1`](../scripts/install-runtime.ps1) use, so there is one copy rather than one per installation route. It then reports when `PATH` resolves `vibe-agent` to a different build, because a shadowed install is invisible and its symptom is a hook behaving like a version you replaced.

Three failures this closes, all of which happened:

- A binary predating `post-tool-use` kept answering the other five hooks and refused that one. It read as a broken hook rather than an out-of-date install, and nothing compared the two: `make install` writes without checking, and the version string was `dev` on both sides so a version comparison proved nothing. A local build now stamps its commit.
- The three installation routes wrote to three different directories: `GOPATH/bin` here, `~/.local/bin` from the shell installer, and a third under `%LOCALAPPDATA%` from the PowerShell one. A machine ended up holding several binaries at different versions, and `PATH` order decided which the hooks called — so a `make install` could be silently ineffective because a copy installed elsewhere still won. All three now share one location, and all three report when `PATH` resolves `vibe-agent` to a different build.
- On Windows, `install` wrote an extensionless file. Git Bash can execute it; nothing else can resolve it by name. Worse, once the `.exe` existed too, a POSIX shell still picked the older extensionless one, so `vibe-agent` meant a different build depending on who asked. `make install` now appends `GOEXE`, and `doctor` reports the shadow if an old file is still there.

`doctor` reads the events every host config in the workspace registers, then asks the binary on `PATH` which ones it handles. It asks that binary rather than reporting its own list, because the process running `doctor` is not necessarily the one answering hooks — and that difference is the whole failure.

Regenerate the golden manifest after an intentional shape change:

```sh
UPDATE_GOLDEN=1 go test ./internal/run/infra/persistence -run TestFreshRunMatchesGolden
```

## Web UI (loopback)

`vibe-agent web` serves the control-plane viewer on `127.0.0.1` only. One process can register multiple workspace roots; the sidebar switches the active root without a restart. The command blocks in the foreground; press Ctrl+C to stop gracefully and remove `.agent-state/web.json`.

Long-running commands (`web`, `hook`, `mcp serve`) write structured logs to a sibling `logs/` directory next to the install `bin/` folder (for example `~/.local/logs/web.log`), with tinted console output. Set `VIBE_LOG_LEVEL` (`debug`, `info`, `warn`, `error`) or `VIBE_LOG_DIR` to override the directory. Error records include a redacted stack trace in the JSON file.

```sh
vibe-agent web --workspace . --toolkit /path/to/vibe-agent --port 3080
vibe-agent web --workspaces /path/other-repo,/path/another-repo
```

The session composer supports `/command` and `@skill` autocomplete from `.ai-agents` ROUTER tables, a loopback file attach browser with redacted previews, and live trajectory updates over SSE (HTMX poll remains the fallback).

## Try it

```sh
vibe-agent goal "add webhook idempotency"
vibe-agent research "compare RAG chunking strategies"
vibe-agent auto init
vibe-agent auto "add webhook idempotency"
vibe-agent run status --slug add-webhook-idempotency
```

`goal`, `research`, and `auto` take the objective as plain text and derive slug and graph. `run start` accepts the same plain-text form; `--slug` and `--graph` remain for scripts.

State lands under `.agent-state/runs/<date>/<slug>/<version>/manifest.json` with an append-only `events.ndjson` beside it. Both are gitignored in consumer workspaces.

## Audit code slop

`vibe-agent slop audit [path]` scans source-like text across a codebase for signals that often show up in low-quality AI-generated patches: unfinished markers, empty declarations, ignored call results, swallowed error branches, debug output, placeholder aborts, AI-tell filler comments, parse errors, oversized files, and repeated non-trivial lines.

```sh
vibe-agent slop audit .
vibe-agent slop audit . --json
vibe-agent slop audit . --fail-on 49
```

The built-in scanner runs without external tools and reports files, lines, languages, tree-sitter parse counts, parser basis, and scoring basis. Language detection comes from [`go-enry`](https://pkg.go.dev/github.com/go-enry/go-enry/v2), the Go port of GitHub Linguist, instead of a local extension table. Syntax parsing uses [`gotreesitter`](https://pkg.go.dev/github.com/odvcencio/gotreesitter), a pure-Go tree-sitter runtime with bundled grammar metadata, so the runtime stays `CGO_ENABLED=0` and still handles mixed repos such as Go, Python, TSX, Vue, YAML, Dockerfiles, PHP, Rust, and Zig. Unknown non-binary text files are still scanned as `Text`. Vendor, generated, binary, image, and sensitive local config files are skipped before scoring.

The score is weighted finding density per KLOC, capped at 100. It is a review signal, not proof that code is correct. The command does not spawn external linters from user-controlled paths; teams that want Semgrep, ast-grep, or benchmark gates should add them as repo-owned verifier commands.

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

## Reading a page without reading the page

`vibe-agent fetch <url|path>` retrieves a source and prints the text, without the
markup, scripts, and navigation that are most of a page. Measured on
`code.claude.com/docs/en/hooks`: **94% smaller** than the raw response, and the
second ask costs no request at all.

```sh
vibe-agent fetch https://example.com/docs   # clipped to the default budget
vibe-agent fetch ./notes.md --budget 8000
vibe-agent fetch <url> --json               # the whole document
vibe-agent fetch <url> --refresh            # ignore the cache
```

Documents are cached under `.agent-state/fetch/`, addressed by a hash of the
source. Output is clipped to a budget and says how many lines were left: an agent
told 400 lines remain can ask for them, and one told nothing assumes it read the
page. Silent truncation is what makes agents assert things about content they
never saw.

Nothing here summarizes. Extraction deletes markup and boilerplate and keeps the
author's words, so what an agent reads is what the page said.

PDF, DOCX, and the rest are refused by name rather than attempted. Their bytes
emitted as text cost a great many tokens of mojibake that the agent cannot detect,
and converting them properly needs an extractor this module has no dependency for.

Three libraries do the work, and none of the HTML is parsed here:

| Stage | Library |
|---|---|
| Parse to a tree, WHATWG algorithm and its error recovery | [`golang.org/x/net/html`](https://pkg.go.dev/golang.org/x/net/html) |
| Strip navigation, header, footer, aside | [`codeberg.org/readeck/go-readability/v2`](https://pkg.go.dev/codeberg.org/readeck/go-readability/v2) |
| Render, with tables and code blocks intact | [`html-to-markdown/v2`](https://pkg.go.dev/github.com/JohannesKaufmann/html-to-markdown/v2) |

All three are pure Go, so `CGO_ENABLED=0` still holds. They cost about 6MB of
binary, 7.6MB to 13.6MB stripped.

**The first version tokenized HTML by hand and that was a mistake worth
recording.** HTML is not a regular language: attribute values hold `>`, script
bodies hold `<`, comments and CDATA nest, and browsers apply an error-correction
algorithm to all of it. That version passed every test in this package and
returned an empty body for the first real page it met, because JavaScript is full
of `<` and one desync swallowed the document. Tables then arrived as `timeout30s`,
two facts glued into something that reads like one. A spec-compliant parser is not
a convenience here; it is the difference between working and appearing to work.

### Images, PDFs, video, and anything else that is not text

An illustration, a screenshot, a spec PDF, a demo video: ordinary things to want
from a page. Refusing them was the first answer here and it was too blunt, since
it left the runtime unable to help at all.

They are retrieved and handed back as a **path**, never as bytes. That is the
same rule as everywhere else in this package: the payload stays out of the
context window and the handle goes in. An image inlined as bytes is tens of
thousands of tokens of something no model reads that way, and the host already
has a file reader that handles images and PDFs properly.

```
$ vibe-agent fetch https://www.python.org/static/img/python-logo.png
https://www.python.org/static/img/python-logo.png is image/png, 15770 bytes,
saved to .agent-state/fetch/assets/20e72af68b277b1e.png. It is not text, so its
bytes are deliberately not printed: open the path with your own file reader...
```

`Status` is `asset`, and `LocalPath` carries the path on its own for a caller
that wants the field rather than the sentence. A local file is named rather than
copied: it is already somewhere openable, and duplicating it would give the
reader two paths for one file.

Nothing here carries a list of formats or extensions. `http.DetectContentType`
implements the sniffing algorithm browsers use, `mime.TypeByExtension` and
`mime.ExtensionsByType` come from the system's own database, and the text-or-not
decision reads the media type's own category. A hardcoded table is wrong the
moment a new format appears, and one always appears.

Media elements get the same treatment inside a page: `video`, `audio`, `source`,
`embed`, and `track` are rewritten as links before conversion, because the
markdown converter has no rule for them and would otherwise drop the URL a
reader came for. Relative URLs resolve against the page they came from, so an
image or a linked file can be fetched afterwards without reconstructing the
origin by hand.

### When a fetch does not get the page

Three ways a request comes back with something that is not the document, and none
of them announce themselves in the status line:

| What happened | How it is caught | What is returned |
|---|---|---|
| Bot check answered `200` | Title signature **and** a body signature together | Refused. Reading "Verifying you are human" as the answer is the failure that cannot be recovered from |
| `403`/`429` | Status | Refused, naming a likely bot check or rate limit |
| Client-rendered page | No text at all, or text far below the markup around it | Returned with `status: empty` or `thin` and "do not answer from this document" |

`Document.Status` is a field, not prose, because the caller that most needs it is
a program deciding whether to answer. `--json` carries it.

Two signatures are required for a bot check, never one. "Just a moment" is a
legitimate page title and "verifying" is a word documentation uses; pairing a
title marker with a body marker such as `/cdn-cgi/challenge-platform` is what
stops the detector eating a guide about the very thing it detects.

**There is no bypass here and there will not be one.** Working around a bot check
is a different activity from reading documentation. What the runtime does instead
is name a route that works: on a failure it probes the origin for `/llms.txt` and
`/llms-full.txt`, a community convention that Anthropic, Cursor, and Vercel
publish, served as text and usually outside the wall. A probe that 404s offers
nothing rather than a guess, and a `200` that returns HTML is treated as a
catch-all rather than a route, since a single-page app answers every path with
its shell.

The other honest answer, when there is no text route: **no AI crawler runs
JavaScript, this one included.** Handing the URL back to the model's own fetch
tool does not help, because that tool has the same limitation. Only a
browser-driving tool does, and that is what the message says.

Documents expire from the cache after `CacheLife`, a day. Documentation changes,
and a cache with no expiry answers a question about today's API from whenever the
page was first read, with nothing in the output to say so.

Two settings differ from the library defaults. Escaping is off, because nothing
renders this output — a model reads it, and `a &lt; b` spends three tokens to say
`<`. The table plugin is on, because it is not on by default. Link targets are
kept: an agent that cannot see where a link goes has to guess a URL, and the token
cost of a link is bounded where a guess is not.

Sources, measurements, and the parts deliberately not built:
[`.ai-agents/references/token-efficiency.md`](../.ai-agents/references/token-efficiency.md).

## Contract with the schemas

[`schemas/run-state.schema.json`](../schemas/run-state.schema.json) is the contract; this module is one implementation of it. Two tests hold them together:

- `TestFreshRunMatchesGolden` pins what `Save` emits.
- `scripts/check-schemas.py` validates that golden file against the schema.

Without both, the writer and the contract drift and nothing notices until a run fails to load.

[`schemas/check-plan.schema.json`](../schemas/check-plan.schema.json) is held the same way: `internal/checkplan` is one implementation, and `check-schemas.py` validates this repo's real [`vibe-checks.yaml`](../vibe-checks.yaml) against the schema. A plan the Go loader accepts and the schema rejects is a drift the first stalled run would find otherwise.
