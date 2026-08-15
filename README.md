# vibe-agent

vibe-agent is a shared toolkit of AI-agent assets: skills, subagents, commands, routers, hooks, runtime graphs, and validation scripts. It is meant to be mounted into other repositories, while each consumer repo keeps its own product rules in `AGENTS.md`.

## Before you install

You need `git` and `curl`. **Go is not required** - the installer downloads a published runtime binary for your platform and only falls back to building from source when no release is reachable.

Some hook scripts still run under `python3` (3.8 or newer). They use the standard library only, so there is nothing to `pip install`, but they need a real interpreter on `PATH`. On a bare Windows install `python3` resolves to an App Execution Alias that opens the Microsoft Store rather than running anything, and those hooks then fail without saying so.

Windows users run the PowerShell installers below; the `sh` ones work under Git Bash or WSL.

## Install

Run these from the toolkit checkout unless the command says it is for a consumer repo.

### Global install

Global install puts shared assets under your home directory with the `vibe-` prefix, for example `vibe-doctor` and `vibe-research`, and installs the runtime binary.

| Shell | Command |
|---|---|
| PowerShell | `powershell -ExecutionPolicy Bypass -File scripts/install-global.ps1` |
| Bash, macOS, Linux, Git Bash, WSL | `sh scripts/install-global.sh` |

| Flag | PowerShell | What it does |
|---|---|---|
| `--dry-run` | `-DryRun` | Print what would change, write nothing. |
| `--check` | `-Check` | Report drift between the installed copies and this checkout, then exit. |
| `--uninstall` | `-Uninstall` | Remove exactly what this script installed, tracked in its own manifest. Nothing else is touched. |
| `--prefix P` | `-Prefix P` | Change the namespace prefix. Default `vibe-`. |

Permissions and hooks are deliberately left out. Applying this repository's policy to every unrelated project on your machine is your decision, not a side effect of installing markdown; run the workspace install in a project to get those.

### Workspace install

Workspace install wires one repository to `.ai-agents` and the tool-specific folders, including permissions and hooks.

| Shell | Command |
|---|---|
| PowerShell | `powershell -ExecutionPolicy Bypass -File scripts/link-ai-agents.ps1` |
| Bash, macOS, Linux, Git Bash, WSL | `bash scripts/link-ai-agents.sh` |

| Flag | PowerShell | What it does |
|---|---|---|
| `--workspace DIR`, `-w` | `-WorkspaceRoot DIR` | Where `.claude`, `.cursor`, `.opencode` are created. Defaults to the directory holding `scripts/`. |
| `--assets DIR`, `-a` | `-AssetsRoot DIR` | The folder holding skills, agents, and commands. Defaults to `<toolkit>/.ai-agents`. |

`LINK_WORKSPACE` and `LINK_ASSETS` set the same two values when the matching flag is omitted.

### Consumer repo install

If this toolkit is mounted in another repo as `.vibe-agent`, run from the consumer repo root:

```powershell
powershell -ExecutionPolicy Bypass -File .vibe-agent/scripts/link-ai-agents.ps1 -WorkspaceRoot $PWD -AssetsRoot (Join-Path $PWD '.vibe-agent\.ai-agents')
```

```bash
bash .vibe-agent/scripts/link-ai-agents.sh --workspace "$PWD" --assets "$PWD/.vibe-agent/.ai-agents"
```

### Runtime binary on its own

The global installers call this for you. Run it directly to pin a version, to rebuild, or to install somewhere else.

```bash
bash scripts/install-runtime.sh            # latest release, else build from source
bash scripts/install-runtime.sh v0.1.0     # a specific version
```

| Variable | What it does |
|---|---|
| `VIBE_FROM_SOURCE=1` | Always build from `runtime/` instead of downloading. Needs Go. |
| `VIBE_INSTALL_DIR` | Where the binary lands. Default `$HOME/.local/bin`. |
| `VIBE_SKIP_RUNTIME` | Set for the global installers to install assets only. |
| `VIBE_REPO` | Release source. Default `ducnd58233/vibe-agent`. |

The runtime is optional by design: without it every hook is a quiet no-op and the markdown assets work as before, so a failed download never leaves a half-installed toolkit.

## How to type a command in each tool

**Three of the four supported tools use `/`. Codex CLI uses `$`.** Getting this wrong is the most common first-run problem, so it is spelled out per tool rather than left to a footnote.

Global install writes commands with the `vibe-` prefix. Workspace install writes them unprefixed, for that repo only. Both forms can exist at once.

| Tool | After global install | After workspace install | Worked example |
|---|---|---|---|
| **Claude Code** | `/vibe-<command>` | `/<command>` | `/vibe-review src/api` |
| **Cursor** | `/vibe-<command>` | `/<command>` | `/vibe-review src/api` |
| **opencode** | `/vibe-<command>` | `/<command>` | `/vibe-review src/api` |
| **Codex CLI** | `$vibe-<command>` | `$<command>` | `$vibe-review src/api` |

Codex CLI 0.147.0 removed custom slash prompts, so its commands are installed as skills instead and the name is typed as ordinary prompt text beginning with `$`. Neither `/vibe-review` nor `/prompts:vibe-review` works there. Everything after the command name is passed through as its arguments, in every tool.

## Commands

| Command | What it does | Use case |
|---|---|---|
| `goal` | Runs the full delivery loop. | Take one objective through clarify, spec, plan, build, review, and ship. Needs the runtime. |
| `spec` | Writes the work spec first. | Capture scope, acceptance criteria, risks, and open questions before coding. |
| `plan` | Breaks a spec into tasks. | Turn an approved spec into ordered tasks with checks and likely files. |
| `build` | Implements the next task. | Build one planned task on its own branch with tests. Needs the runtime. |
| `test` | Runs a TDD or proof loop. | Start with a failing test, prove a bug, or run focused validation. Needs the runtime. |
| `review` | Reviews a diff from five angles. | Check correctness, readability, architecture, security, and performance. Needs the runtime. |
| `ship` | Makes the pre-ship call. | Run the final review and return GO or NO-GO with blockers named. Needs the runtime. |
| `research` | Gathers cited evidence. | Answer one scoped research question with sources. |
| `analyze` | Turns evidence into a recommendation. | Compare options, tradeoffs, confidence, and likely next steps. |
| `investigate` | Runs parallel evidence lanes. | Use investigator, analyst, and source-auditor lanes on a multi-part question. |
| `doctor` | Checks AI asset health. | Check routers, generated views, hooks, permissions, runtime wiring, and drift. |
| `harden` | Audits AI asset safety. | Review permissions, hooks, tool boundaries, and secret-handling risks. |
| `code-simplify` | Cleans up code without behavior change. | Cut complexity while keeping tests green and behavior unchanged. |
| `design` | Builds or audits UI. | Work registry-first, then check accessibility, design drift, and rendered evidence. |

Preconditions for each command, and the file behind it, are in [`.ai-agents/commands/ROUTER.md`](.ai-agents/commands/ROUTER.md).

## Guards, and customising them

After a workspace install, the runtime inspects each file an agent writes and reports what it finds: credentials heading somewhere readable, personal data in a log, raw colour literals, tests that cannot fail. The built-in rules travel inside the binary, so a fresh install guards something with no setup.

Targeting is by language category rather than by file extension, so a stack nobody listed is still covered - Terraform, Dart, Objective-C, Ruby, and PHP are all read without appearing in any list, and documentation is left alone because prose is a category of its own.

See what is running here, then start a plan of your own:

```bash
vibe-agent guards list          # every guard, what it reads, and its checks
vibe-agent guards init          # writes .ai-agents/guards.yaml
```

The file `init` writes is entirely commented out, so nothing changes until you edit it. Uncomment a block to add a rule for your stack, switch off a single check by id, retarget a guard at other languages, or disable one outright. Keep the file in git: weakening a guard should be a diff someone reviews.

## The runtime

`vibe-agent` is a Go binary the delivery commands require and refuse to run without. It also holds run state, workflow graphs, memory, and the hook handlers.

Check an install with `vibe-agent doctor`. The command surface, the evidence rules behind it, and what it stores where are in [`runtime/README.md`](runtime/README.md).

## Where things live

Edit assets under `.ai-agents/`. The `.claude/`, `.cursor/`, `.codex/`, `.opencode/`, and `.agents/` folders are generated views, not the source of truth.

The full layout, the authoring rules, the checks table, and the clone and mounting steps are in [`.ai-agents/AUTHORING.md`](.ai-agents/AUTHORING.md). Read it when you are creating an asset or wiring a repo.
