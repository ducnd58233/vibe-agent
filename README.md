# vibe-agent

Shared agent skills, slash commands, hooks, and a Go runtime (`vibe-agent`) that consumer repos mount under `.vibe-agent/`. Domain rules live in each repo's own `AGENTS.md`.

## Install

You need `git` and `curl`. Go is only required to build the runtime from source. Some hooks call `python3` (3.8+, stdlib). On Windows, disable the Microsoft Store `python3` alias or those hooks fail silently.

There are two install shapes, and they're not alternatives — most setups end up using both:

| | Global | Workspace |
|---|---|---|
| What it is | One shared copy for the whole machine | A copy mounted inside one repo — think git submodule or vendored subfolder, not a system package |
| Where it lands | `~/.vibe-agent`, `~/.claude/skills`, … | `.ai-agents/` inside whichever repo it's linked into |
| Commands | Prefixed: `/vibe-goal`, `$vibe-goal` | Unprefixed: `/goal`, `$goal` |
| Followed by | Every project on the machine | Only the repo it's linked into |
| Run it | Once per machine | Once per repo that should carry its own copy of the assets, permissions, and hooks — including this repo, if you're working on the toolkit itself |

| Goal | PowerShell | Bash |
|---|---|---|
| Global: put commands on PATH (`vibe-goal`, `vibe-agent`, …) | `powershell -ExecutionPolicy Bypass -File scripts/install-global.ps1` | `sh scripts/install-global.sh` |
| Workspace: link *this* checkout (`.claude`, `.cursor`, `.opencode`) | `powershell -ExecutionPolicy Bypass -File scripts/link-ai-agents.ps1` | `bash scripts/link-ai-agents.sh` |

### Workspace install in another repo

To use vibe-agent from a product repo without vendoring it, mount this toolkit as a git submodule at a
path of your choosing (`.vibe-agent/` below), then run the *same* workspace link script from the row
above, pointed at the consumer repo instead of at itself:

```bash
git submodule add git@github.com:ducnd58233/vibe-agent.git .vibe-agent   # once, from the consumer repo root
bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"
vibe-agent doctor   # repeat until OK
```

`--workspace` is the consumer repo's root, where `.claude`, `.cursor`, and friends get written.
`--assets` always points at `<toolkit-root>/.ai-agents`, wherever the toolkit is mounted. The consumer
repo stays its own repository and the source of product code; vibe-agent only supplies the shared
assets, the same way a submodule supplies shared library code without becoming part of your app.

## Add skills

Install a community Agent Skill into Claude Code, Codex, Cursor, and/or opencode. This is not the
same as editing this toolkit: do not copy third-party trees into `.ai-agents/`. `vibe-agent` must be
on PATH (global install above) and `npx` must be available.

```bash
# All four hosts, user-level:
vibe-agent skills add <owner/repo> -g -y

# One host, one skill from a multi-skill repo:
vibe-agent skills add <owner/repo> -a cursor -g -y --skill <name>

# Same install without the wrapper:
npx skills add <owner/repo> -a claude-code -a codex -a cursor -a opencode -g -y
```

`-a` may be `claude-code`, `codex`, `cursor`, or `opencode`. Repeat `-a` for a subset. `-g` is
user-level; omit it to install into the current project.

Skills written for one host often carry extra frontmatter. Report it; files are not rewritten. This
command never writes MCP configs or host hooks.

```bash
vibe-agent skills convert-report ~/.agents/skills/<name>
```

Where files land, the Codex `~/.codex/skills` trap, and host-only keys: [Third-party Agent Skills in AUTHORING.md](.ai-agents/AUTHORING.md#third-party-agent-skills-not-this-toolkit).

To *write* a skill this repo ships, edit [`.ai-agents/skills/`](.ai-agents/skills) and follow
[AUTHORING.md](.ai-agents/AUTHORING.md). That is not `skills add`.

## How commands look in your host

Claude Code, Cursor, and opencode use `/`. Codex CLI uses `$`. A global install adds the `vibe-` prefix; a workspace install does not.

| Tool | Global example | Workspace example |
|---|---|---|
| Claude Code, Cursor, opencode | `/vibe-goal` | `/goal` |
| Codex CLI | `$vibe-goal` (skill) | `$goal` |

Codex does not load custom `/prompts`; this kit installs commands as skills. Authoring and clone steps: [`.ai-agents/AUTHORING.md`](.ai-agents/AUTHORING.md). Full command list: [`.ai-agents/commands/ROUTER.md`](.ai-agents/commands/ROUTER.md).

## Supported coding hosts

The runtime hooks session start, prompt submit, pre-tool gates, post-tool journaling, and stop for every host below. Delivery commands (`/goal`, `/build`, `/test`, `/ship`, `/auto`) need the `vibe-agent` binary on PATH; hook wiring alone is not enough.

| Host | Binary | Hook config | Status |
|---|---|---|---|
| Claude Code | `claude` | `.claude/settings.json` | Verified |
| Cursor | `cursor-agent` | `.cursor/hooks.json` | Verified |
| Codex CLI | `codex` | `.codex/config.toml` | Verified |
| opencode | `opencode` | `opencode.json` | Verified |
| Google Antigravity | `antigravity` | `.agents/hooks.json` | Hook wiring shipped; payload fields UNVERIFIED until observed on a live host |
| Kimi | `kimi` | `.kimi/hooks.toml` (merge into `~/.kimi/config.toml`) | Hook wiring shipped; Kimi reads user config only |
| Muse | `muse` | `.muse/hooks.json` | Hook wiring shipped; run `muse hooks trust` after install |

`bash scripts/link-ai-agents.sh` creates Antigravity, Kimi, and Muse hook stubs when those files are missing. `vibe-agent doctor` scans all seven configs and reports wiring status. Generated hook contracts: [`.ai-agents/references/host-hook-contracts.md`](.ai-agents/references/host-hook-contracts.md).

Codex and Antigravity also get command skills under `.agents/skills/` (Agent Skills layout). Cursor and Claude keep commands under their own generated views.

## Three ways to work

Pick one path. They compose the same assets; what changes is who drives each gate and which runtime graph starts.

```mermaid
flowchart TB
  subgraph manual [Step by step: you invoke each slash command]
    M1["/spec"] --> M2["/research"]
    M2 --> M3["/plan"]
    M3 --> M4["/build"]
    M4 --> M5["/test"]
    M5 --> M6["/review"]
    M6 --> M7["/ship"]
  end

  subgraph runtime [Runtime graph: one prompt drives the sequence]
    G["/goal or /auto"] --> GD[goal-delivery]
    AR["/auto research or /research"] --> RD[researcher-delivery]
  end
```

### Step by step (you run each phase)

Use individual slash commands when you want to stop and review between stages.

```mermaid
flowchart LR
  spec["/spec"] --> research["/research"]
  research --> plan["/plan"]
  plan --> build["/build"]
  build --> simplify["/code-simplify"]
  simplify --> test["/test"]
  test --> review["/review"]
  review --> ship["/ship"]
```

Skip `/research` when the repo already has the facts. `/code-simplify` is optional. `/build` through `/ship` need the runtime.

| Command | Role |
|---|---|
| `/spec` | Write the spec. |
| `/research` | Citation-first digest (see [Research](#research) below). |
| `/plan` | Tasks from an approved spec. |
| `/build` | One planned task on its own branch. Needs the runtime. Never merges to `main`. |
| `/test`, `/review`, `/ship` | Proof, review, merge gate. Need the runtime. |

`/design`, `/analyze`, and `/investigate` are optional when the work needs extra passes.

### One outcome (`/goal`)

Same delivery pipeline as the step-by-step list, but the runtime graph walks the sequence and pauses at checkpoints.

```mermaid
flowchart LR
  intake[intake] --> research{research?}
  research -->|yes| lit[research]
  research -->|no| spec[spec]
  lit --> spec
  spec --> plan[plan]
  plan --> build[build]
  build --> test[test]
  test --> ship[review / ship]
  ship --> merge[merge]
  merge --> done[done]
```

Human gates on spec and plan under `/goal`; `/auto` can skip them when docs pass structural checks.

```text
/goal Add host token counts to the Chat toolbar.
```

The host agent runs `vibe-agent goal "<your text>"`. Slug and graph are derived; do not pass `--goal`, `--graph`, or `--slug`. Rules: [`.ai-agents/commands/goal.md`](.ai-agents/commands/goal.md).

### Not code (`/task`, `/tutor`)

For a report, an analysis, a document, a data job, an ops step, or a message to other people, there is no branch or pull request to gate on. `/task` runs the `task-delivery` graph instead: you approve a spec with written acceptance rows, the work is checked row by row, and you approve the exact delivery before anything is sent, posted, paid, or shared.

```text
/task Summarize the vendor renewal terms in the attached contract and draft a reply to the account manager
```

The host agent runs `vibe-agent task "<your text>"`. `/auto task` reaches the work on its own and stops at the delivery gate every time, because the delivery step is the one a person has to approve. Rules: [`.ai-agents/commands/task.md`](.ai-agents/commands/task.md).

To study a subject yourself, `/tutor` (or `/goal tutor`) tests what you can do instead of telling you what to know: it asks first, plans around your deadline, teaches by questions, and schedules reviews on computed dates in a study record you own. The run remembers where you are between sessions and ends only when every topic is tested as learned. Rules: [`.ai-agents/commands/tutor.md`](.ai-agents/commands/tutor.md).

### Unattended (`/auto`)

Same graphs and checks as `/goal`, but approval gates close on evidence instead of waiting for you. Requires the runtime and a workspace opt-in file.

```bash
vibe-agent auto init                              # writes .agent-state/auto.yaml (merge: false)
vibe-agent auto "Add webhook idempotency"         # goal-delivery
vibe-agent auto research "Compare RAG chunking"   # researcher-delivery
```

| Gate | `/goal` | `/auto` |
|---|---|---|
| Spec and plan | You approve | Skipped when the docs pass structural checks |
| Merge to `main` | You approve | Only if `auto.yaml` says `merge: true` and CI, tests, lint, and `/ship` already pass |
| Danger list (migrations, prod writes, credentials, …) | You decide | Stops every time |

No opt-in file, or `merge: false`, means auto stops at a green PR and you merge manually. Rules: [`.ai-agents/commands/auto.md`](.ai-agents/commands/auto.md).

## Research

`/goal` and `/auto` pick the graph in one of two ways, and print which. Name the kind of work first (`delivery`, `research`, `experiment`, `task`, or `tutor`) and that graph starts. Name nothing and the runtime reads the objective with fixed rules, then prints the words that decided it on a `chosen` line, so a wrong guess is visible and one command fixes it.

| You write | Graph | Notes |
|---|---|---|
| `/goal Add retry to the webhook dispatcher` | `goal-delivery` | Code words, or no routing words: the default |
| `/goal task Summarize the vendor contract` | `task-delivery` | A report, document, data job, ops step, or message |
| `/goal research Compare chunking strategies` | `researcher-delivery` | Literature, then experiment and findings |
| `/goal experiment Benchmark two embedding models` | `researcher-delivery` | The same graph, entered for an experiment |
| `/goal tutor Linear algebra for my exam` | `study-delivery` | A learner studying over many sessions. No `/auto` form |
| `/goal Implement the export endpoint and write a report on how it works` | `goal-delivery` | A code change and a non-code deliverable: the report is delivered after the last code task. `--with-task` forces this |

An objective that could be two kinds (research plus a document to write) is a question: the runtime asks which, and does not guess. `/auto` acts on a guess only when the signal is strong, and otherwise asks for the word. To force the default, write `delivery` first.

```mermaid
flowchart LR
  lit[literature] --> app[applicability]
  app --> hyp[hypothesis]
  hyp --> design[experiment design]
  design --> run[experiment run]
  run --> mon[monitor]
  mon --> met{metrics OK?}
  met -->|no| hyp
  met -->|yes| find[findings]
  find --> write[writeup]
  write --> done[done]
```

`/research`, `/experiment`, `/goal research`, `/goal experiment`, and the same words after `/auto` use this graph. An objective with code words uses goal-delivery (diagram above) instead; research there is at most one phase before spec.

| Intent | Slash (global) | Host runs | Graph |
|---|---|---|---|
| Cited digest; you approve applicability and design | `/vibe-research "<topic>"` | `vibe-agent research "<topic>"` | `researcher-delivery` |
| Literature, experiment, metric loop (unattended) | `/vibe-auto research "<topic>"` | `vibe-agent auto research "<topic>"` | `researcher-delivery` |
| Research as one phase, then spec, build, and PR | `/vibe-auto "<outcome>"` | `vibe-agent auto "<outcome>"` | `goal-delivery` |

Examples:

```text
/vibe-research Chunking strategies for RAG on legal PDFs

/vibe-auto research Compare embedding models; loop until recall@10 >= 0.85

/vibe-auto Add a small LLM inference module: research quantization, spec, implement, test, open PR
```

The third example starts **goal-delivery** (optional research node, then spec and ship). Only the second runs the experiment monitor and `experiment/METRICS.json` loop that can route back to hypothesis.

More: [`.ai-agents/commands/research.md`](.ai-agents/commands/research.md), graph [`.ai-agents/graphs/researcher-delivery.yaml`](.ai-agents/graphs/researcher-delivery.yaml).

## Watch a run

```bash
vibe-agent run status --slug <slug>
vibe-agent web --open    # http://127.0.0.1:1411/, local only
```

Runtime flags and internals: [`runtime/README.md`](runtime/README.md).

## Where to edit

Edit sources under [`.ai-agents/`](.ai-agents). Paths like `.claude/`, `.cursor/`, and `.codex/` are generated views; re-run the link script after changes.

To install someone else's skill, use [Add skills](#add-skills), not this folder.
