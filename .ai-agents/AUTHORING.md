# Authoring and setup

<context>

Everything an agent needs **only when creating toolkit assets or setting a repo up**. It lived in the
root [`AGENTS.md`](../AGENTS.md), which every session loads in full, so a rule needed once per month
was costing tokens on every turn. AGENTS.md keeps the rules that govern behavior each turn and points
here for the rest.

Read this file when you are: adding or changing an asset, wiring a harness, or mounting the toolkit
into a consumer repo. Skip it otherwise.
</context>

## Project layout

<context>

| Location | Role |
|----------|------|
| [`.ai-agents/README.md`](README.md) | Index of shared skills, agents, commands, stack profiles, references, and hooks. |
| [`.ai-agents/ROUTER.md`](ROUTER.md) | Master router - start here to pick which asset family applies. |
| [`.ai-agents/*/ROUTER.md`](skills/ROUTER.md) | Per-folder routers - intent to concrete asset; **must** stay in sync when assets change. |
| [`.ai-agents/*/TEMPLATE.md`](skills/TEMPLATE.md) | Authoring contracts for folders that define one. |
| [`.ai-agents/PERMISSIONS.md`](PERMISSIONS.md) | Permissions and authority mapping to [`.claude/settings.json`](../.claude/settings.json), hooks, and subagent `tools:`. |
| [`.ai-agents/skills/`](skills) | Canonical skills (`SKILL.md` per folder), stack-agnostic by default. |
| [`.ai-agents/agents/`](agents) | Subagent/persona definitions (`*.md`). |
| [`.ai-agents/commands/`](commands) | Slash-command prompts (`*.md`). |
| [`.ai-agents/references/`](references) | Generic checklists and patterns. |
| [`.ai-agents/stack-profiles/`](stack-profiles) | Repo-pinned stack and domain profiles. |
| [`.ai-agents/graphs/`](graphs) | Executable workflow graphs (`*.yaml`). |
| [`.ai-agents/hooks/`](hooks) | Shared hook scripts. |
| [`schemas/`](../schemas) | JSON Schema contracts for graphs, run state, and memory records. |
| [`runtime/`](../runtime) | Go control plane. Required by the delivery pipeline. |
| [`.claude/`](../.claude), [`.cursor/`](../.cursor), [`.opencode/`](../.opencode), [`.codex/`](../.codex), [`.agents/`](../.agents) | Harness config plus generated links, produced by `scripts/link-ai-agents`. |
| [`opencode.json`](../opencode.json), [`CLAUDE.md`](../CLAUDE.md), [`CURSOR.md`](../CURSOR.md) | Per-harness entry points. |
</context>

## Authoring rules (MUST)

<required>

- **Follow the folder's `TEMPLATE.md`** where present when creating a skill, subagent, command, hook,
  reference, or stack profile.
- **Update that folder's `ROUTER.md` in the same change** after creating, renaming, or deleting any
  asset under `skills/`, `agents/`, `commands/`, `hooks/`, `references/`, `stack-profiles/`, or
  `graphs/`.
- **Do not restate a rule that already has a home.** Link to it. The stack-detection rule, the
  permissions defaults, the git gates, and the runtime requirement each live in exactly one place;
  copies drift and the copy is what a reader trusts.
- **Tool naming and currency:** shared skills, references, and commands stay tool-agnostic and
  describe the capability, not a product. Concrete tools, packages, and libraries are named only in
  [`stack-profiles/`](stack-profiles), and even there as non-exhaustive examples the agent must
  verify against current official docs. Prefer detecting what the repo uses over any hardcoded list.
  See [`stack-profiles/TEMPLATE.md`](stack-profiles/TEMPLATE.md).
- **Tool-specific rules:** Cursor rule files live in [`.cursor/rules/`](../.cursor/rules) and are not
  interchangeable with Claude rules without editing.
- **Permissions:** after changing tool or path requirements, align
  [`.claude/settings.json`](../.claude/settings.json) and [`PERMISSIONS.md`](PERMISSIONS.md). Deny
  overrides allow.
</required>

## XML section tags

<context>

Always-loaded files wrap their sections in XML tags. This is **prompt content, not a file format**.
Anthropic's prompt guidance says XML tags "help Claude parse complex prompts unambiguously,
especially when your prompt mixes instructions, context, examples, and variable inputs", and advises
"consistent, descriptive tag names across your prompts". Nothing in it is about files on disk.

Converting assets to HTML or XML files was considered and rejected on evidence: Claude Code requires
a `SKILL.md`, and Cursor requires `.mdc` and ignores a plain `.md` in `.cursor/rules`. Neither
documents HTML or XML support. Files stay Markdown; only the body gains tags.

### The tag set

Bounded on purpose. A vocabulary nobody can remember gets used inconsistently, and inconsistent tags
are worse than none: they teach the model a partition that does not hold.

**Charter files** (loaded every turn):

| Tag | Holds |
|---|---|
| `<scope>` | What this repository is and is not |
| `<precedence>` | Which source of rules wins, in order |
| `<always_on>` | Behavior that applies on every turn |
| `<delivery_gates>` | Gates in front of branches, merges, attribution, secrets |
| `<claude_specific>`, `<other_harnesses>` | Harness-specific pointers |

**Asset files** (skills, agents, commands, references):

| Tag | Holds | The question it answers |
|---|---|---|
| `<persona>` | Role, authority, and boundary of a subagent | Who am I acting as? |
| `<prerequisites>` | What must be read, installed, or true before starting | Can I start yet? |
| `<context>` | Background needed to judge well, not to execute | What do I need to understand? |
| `<required>` | Non-negotiable rules. Violating one is a defect | What blocks me? |
| `<rules>` | Operative guidance that judgment may weigh | What should guide me? |
| `<procedure>` | Ordered steps | What do I do, in what order? |
| `<inputs>` / `<outputs>` | The contract at each end | What comes in, what must come out? |
| `<verification>` | How completion is proven, not asserted | How do I know it worked? |
| `<antipatterns>` | Named failure modes and rationalizations | What am I likely to get wrong? |
| `<routing>` | When to use this, and when not to | Is this the right asset? |
| `<escalation>` | Conditions to stop and ask a human | When do I stop? |
| `<references>` | Links out: further reading, related assets, source URLs | Where do I read more? |

**`<required>` versus `<rules>` is the distinction that earns the whole scheme.** A weak model
flattens a document into one list of suggestions. Splitting the blocking rules from the advisory ones
tells it which line it may trade away under pressure and which it may not. If a block would be
correct to violate given a good reason, it is `<rules>`. If violating it is always a defect, it is
`<required>`.

### Rules for using them

- Tags go **after** YAML frontmatter, never before. Frontmatter must stay the first bytes of the file
  or the skill and rule loaders will not parse it.
- **One prose sentence goes above the first tag**, and no tag may be the first body line. This is the
  single exception to total tag coverage below, and it is not a section: it is the description, for
  the loaders that never read frontmatter. Cursor's `.cursor/commands` takes plain Markdown and
  advertises the first body line as-is, so thirteen commands here showed up in Cursor's `/` picker as
  the literal string `<references>` or `<context>` while their frontmatter was valid the whole time.
  `commands/doctor.md` was the one that read correctly, because it happened to open on a sentence.
  `scripts/check-frontmatter.py` check 4 now fails a tag-first body.
- Open and close on their own lines, so the Markdown around them still renders.
- **Every section belongs to a block.** An untagged region between two tagged ones asks the model to
  guess what kind of instruction it is reading, which is the ambiguity the tags exist to remove.
  Every asset under `.ai-agents/` is tagged, including short ones.
- **Nest only for genuine containment, one level deep.** Anthropic's prompting guidance says to
  "nest tags when content has a natural hierarchy", and its example is a container holding items of
  the same kind. A long `<procedure>` whose phases each carry their own `<verification>` is that
  shape. Two categories side by side are not: `<required>` inside `<rules>` invents a question with
  no answer, namely whether the requirement stops applying outside that block.
- **Pick the tag by what the block is, not by what it is called.** A heading reading "Quick
  Reference" that holds code examples is `<rules>`, not `<references>`.
- Use the smallest set that partitions the file. Most assets need three or four, not eleven.
- A tag pair costs roughly six tokens. Measured here, tags cost about as much as the restated
  boilerplate they replaced, so treat the partition as the return, not a smaller file.

`scripts/check-xml-tags.sh` fails on a mispaired tag, an unknown tag, a tag placed above
frontmatter, a tag nested inside itself, or nesting past one level.
</context>

## Checks to run

<verification>

| After changing | Run |
|---|---|
| Any asset under `.ai-agents/` | `bash scripts/check-ai-agents-routers.sh` or `powershell -File scripts/check-ai-agents-routers.ps1` (dependency-free) |
| An always-loaded file or any tagged asset | `bash scripts/check-xml-tags.sh` - pairing, nesting depth, unknown tags, and tags above frontmatter |
| Any asset's YAML frontmatter, or its first body line | `python3 scripts/check-frontmatter.py` - invalid frontmatter does not error, it silently demotes the first body line to the description, and Cursor's command loader demotes it even when the frontmatter is valid |
| Anything, before trusting a harness read | `bash scripts/check-generated-views.sh` - a canonical edit does not reach the harness until the link script re-runs |
| `.ai-agents/graphs/*.yaml` or `schemas/*.json` | `python3 scripts/check-graphs.py` and `python3 scripts/check-schemas.py` |
| [`runtime/`](../runtime) | `cd runtime && make check` |
| `.ai-agents/agents/*.md` or `.ai-agents/commands/*.md` | re-run the link script, then `powershell -File scripts/check-codex-assets.ps1 -Global` so Codex generated agents and best-effort prompt files stay in sync |

The python checks need `python3 -m pip install -r scripts/requirements.txt`. Full table:
[`README.md`](README.md).
</verification>

## After clone

<procedure>

Run `scripts/link-ai-agents.ps1` (Windows) or `scripts/link-ai-agents.sh` (macOS, Linux) so
`.claude`, `.cursor`, `.opencode`, `.agents`, and `.codex/agents` point at or are generated from `.ai-agents`. Codex commands are generated as skills under `.agents/skills`; current Codex CLI does not load custom `/prompts`. The script also installs minimal hook configs when they are absent, installs the git `prepare-commit-msg` attribution hook, and refreshes the runtime binary.

**Re-run it after every asset edit.** On Windows the script writes copies rather than symlinks, so
until it runs again the harness serves the previous text while every other check reports green.
`scripts/check-generated-views.sh` exists because that happened.
</procedure>

## Reuse in a consumer repository

<rules>

- Keep the consumer repo as its own repository and the source of product code.
- Mount this toolkit as a submodule at a chosen path, for example `.vibe-agent`.
- Treat `<toolkit-root>/.ai-agents` as the canonical shared assets path.
- Give the consumer repo its own root `AGENTS.md` with product and domain constraints. It wins over
  this toolkit; see local-first precedence in [`AGENTS.md`](../AGENTS.md).
- Run the link scripts from the submodule with `-WorkspaceRoot` / `--workspace` set to the consumer
  root and `-AssetsRoot` / `--assets` set to `<toolkit-root>/.ai-agents`.
- Treat tool permissions as repository-local policy: adapt `opencode.json`, `.claude/settings.json`,
  and local rules to that repo's layout and risk profile.
</rules>

## Machine-wide install

<procedure>

Installs the assets into the user-level directories, so every project on the machine sees them
without a wrapper workspace and without a submodule. Use it when the toolkit should follow you
rather than a repository, and keep using the link script for repositories that also want the
permissions, hooks, and runtime gates.

| Platform | Run |
|---|---|
| Linux, macOS, WSL, Git Bash | `sh scripts/install-global.sh` |
| Windows | `powershell -ExecutionPolicy Bypass -File scripts/install-global.ps1` |

Both also install the runtime binary, downloading a published release and falling back to building
from source, so a fresh machine needs one command. Set `VIBE_SKIP_RUNTIME` to install only the
markdown. A failed download never fails the install: the assets work without the binary, and the
delivery commands that need the control plane say so.

Both write the same layout and share one manifest, so either can uninstall what the other installed.
The PowerShell port is not duplication for symmetry: Git Bash on Windows accepts `ln -s` and
silently copies, so only PowerShell can produce live symlinks, and only with Developer Mode on.
Each script attempts a link, verifies it, falls back to a copy, and reports which it did.

Everything installs under a `vibe-` prefix, and the prefix carries weight. Claude Code resolves a
skill-name collision toward the personal level, so an unprefixed global install would make this
toolkit override a repository's own skills instead of being the fallback that
[`AGENTS.md`](../AGENTS.md) says it is. Prefixing means the collision never happens.

**Skills need only two directories to reach all four tools**, because the Agent Skills convention is
shared:

| Path | Read by |
|---|---|
| `~/.claude/skills/` | Claude Code, opencode |
| `~/.agents/skills/` | Codex, Cursor, opencode |

Codex reads `$HOME/.agents/skills` and not `~/.codex/skills`, which is easy to get backwards because
an empty `~/.codex/skills` exists on some machines.

| Asset | Installed as | Why that shape |
|---|---|---|
| Skills | linked dirs in the two paths above | The command name is the directory name, so a rename is enough and edits stay live |
| Control plane | `~/.vibe-agent/.ai-agents` | The runtime needs the workflow graphs and hook wiring, and neither is repository-specific |
| Commands | `vibe-<name>.md` in tools that read command folders, plus generated Codex skill adapters | Codex CLI reads skills and does not load custom `/prompts`; no shared command convention exists |
| Subagents | generated copies with `name: vibe-<name>` | A subagent is identified by its frontmatter `name:` and the filename need not match, so renaming namespaces nothing |
| Rules | a marked block in each global instructions file, plus `~/.cursor/rules/vibe-toolkit.mdc` | Codex concatenates global with project rules; Cursor has no global `AGENTS.md`, so it gets an `alwaysApply` rule instead |

**A repository does not need to vendor the toolkit to use the runtime.** `vibe-agent doctor` and the
delivery commands look for `.ai-agents` in this order, most specific first:

1. `--toolkit`, which beats everything
2. the workspace root
3. one directory down, the submodule layout
4. `$VIBE_AGENT_TOOLKIT`
5. `~/.vibe-agent`, written by the global install

Local before global matches the precedence rule in [`AGENTS.md`](../AGENTS.md): a repository that
ships its own assets means them. Without steps 4 and 5, a consumer repo had to vendor the whole
toolkit to obtain two files, and `doctor` failed on a missing `.ai-agents/graphs` in a workspace that
was otherwise set up correctly.

**Hooks are installed per workspace by the link script when a host config file is absent. Permissions remain repository-local policy and are never installed globally.** This repo denies 21 patterns and hooks six events;
applying that to every unrelated repository on the machine is the user's call, made by running the
link script in a specific project.

Subagent files are rewritten, and any asset that could not be symlinked is a copy, so both can go
stale. `--check` compares what is installed against what a fresh install would produce, `--uninstall`
removes exactly what the manifest records and strips only the marked block from a rules file, and
`--dry-run` writes nothing.
</procedure>

## Asset inventory

<context>

There is no list here on purpose. The router files are authoritative, and a second list in prose is
a list that goes stale without anything failing. Start at [`ROUTER.md`](ROUTER.md).
</context>
