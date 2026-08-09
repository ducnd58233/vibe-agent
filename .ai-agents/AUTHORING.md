# Authoring and setup

Everything an agent needs **only when creating toolkit assets or setting a repo up**. It lived in the
root [`AGENTS.md`](../AGENTS.md), which every session loads in full, so a rule needed once per month
was costing tokens on every turn. AGENTS.md keeps the rules that govern behavior each turn and points
here for the rest.

Read this file when you are: adding or changing an asset, wiring a harness, or mounting the toolkit
into a consumer repo. Skip it otherwise.

## Project layout

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

## Authoring rules (MUST)

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

## XML section tags

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

**`<required>` versus `<rules>` is the distinction that earns the whole scheme.** A weak model
flattens a document into one list of suggestions. Splitting the blocking rules from the advisory ones
tells it which line it may trade away under pressure and which it may not. If a block would be
correct to violate given a good reason, it is `<rules>`. If violating it is always a defect, it is
`<required>`.

### Rules for using them

- Tags go **after** YAML frontmatter, never before. Frontmatter must stay the first bytes of the file
  or the skill and rule loaders will not parse it.
- Open and close on their own lines, so the Markdown around them still renders.
- **Do not nest.** A flat partition is the point; nesting rebuilds the ambiguity tags removed.
- **Do not tag a single paragraph.** A tag earns its place when a model needs to act on that block
  alone. Six tags in a forty-line file is noise.
- Use the smallest set that partitions the file. Most assets need three or four, not eleven.
- A tag pair costs roughly six tokens. On a charter file that is free; across the whole corpus it
  adds up, so tag what is genuinely multi-part and leave short files alone.

`scripts/check-xml-tags.sh` fails on an unbalanced tag, an unknown tag, or a tag placed above
frontmatter.

## Checks to run

| After changing | Run |
|---|---|
| Any asset under `.ai-agents/` | `bash scripts/check-ai-agents-routers.sh` or `powershell -File scripts/check-ai-agents-routers.ps1` (dependency-free) |
| An always-loaded file or any tagged asset | `bash scripts/check-xml-tags.sh` - balance, nesting, unknown tags, and tags above frontmatter |
| Anything, before trusting a harness read | `bash scripts/check-generated-views.sh` - a canonical edit does not reach the harness until the link script re-runs |
| `.ai-agents/graphs/*.yaml` or `schemas/*.json` | `python3 scripts/check-graphs.py` and `python3 scripts/check-schemas.py` |
| [`runtime/`](../runtime) | `cd runtime && make check` |
| `.ai-agents/agents/*.md` | re-run the link script, then `powershell -File scripts/check-codex-assets.ps1` so `.codex/agents/*.toml` stays loadable |

The python checks need `python3 -m pip install -r scripts/requirements.txt`. Full table:
[`README.md`](README.md).

## After clone

Run `scripts/link-ai-agents.ps1` (Windows) or `scripts/link-ai-agents.sh` (macOS, Linux) so
`.claude`, `.cursor`, `.opencode`, `.agents`, and `.codex/agents` point at `.ai-agents`. The script
also installs the git `prepare-commit-msg` attribution hook and refreshes the runtime binary.

**Re-run it after every asset edit.** On Windows the script writes copies rather than symlinks, so
until it runs again the harness serves the previous text while every other check reports green.
`scripts/check-generated-views.sh` exists because that happened.

## Reuse in a consumer repository

- Keep the consumer repo as its own repository and the source of product code.
- Mount this toolkit as a submodule at a chosen path, for example `.vibe-agent`.
- Treat `<toolkit-root>/.ai-agents` as the canonical shared assets path.
- Give the consumer repo its own root `AGENTS.md` with product and domain constraints. It wins over
  this toolkit; see local-first precedence in [`AGENTS.md`](../AGENTS.md).
- Run the link scripts from the submodule with `-WorkspaceRoot` / `--workspace` set to the consumer
  root and `-AssetsRoot` / `--assets` set to `<toolkit-root>/.ai-agents`.
- Treat tool permissions as repository-local policy: adapt `opencode.json`, `.claude/settings.json`,
  and local rules to that repo's layout and risk profile.

## Asset inventory

There is no list here on purpose. The router files are authoritative, and a second list in prose is
a list that goes stale without anything failing. Start at [`ROUTER.md`](ROUTER.md).
