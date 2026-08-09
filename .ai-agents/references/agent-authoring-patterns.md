# Agent authoring patterns

Use this reference when creating or reviewing skills, commands, agents, hooks, and stack profiles.

## Principles

<context>

- **One asset, one job:** skills define reusable workflow; agents define isolated perspective; commands define repeatable entrypoints; stack profiles define pinned stack facts.
- **Progressive disclosure:** keep routers and frontmatter concise; put detailed procedures in the asset body; put long checklists in references.
- **Explicit authority:** every agent and hook must state tools, paths, and side effects.
- **Observable completion:** define what “done” means and how to verify it.
- **No persona trees:** commands or users orchestrate; agents do not call agents.
</context>

## Asset design checklist

<verification>

- [ ] Trigger language is concrete enough to route automatically.
- [ ] Inputs and outputs are explicit.
- [ ] Non-goals prevent scope creep.
- [ ] Verification is concrete: command, diff check, cited source, or review checklist.
- [ ] Permissions match actual tool use.
- [ ] Router row is updated in the same change.
</verification>

## When to add an asset

<routing>

Add a new asset only when at least one is true:

- The workflow repeats across repositories.
- The task needs a distinct authority boundary.
- The procedure is fragile enough to benefit from a saved prompt/checklist.
- The detail would bloat always-on instructions.

Do not add an asset just to rename an existing workflow.
</routing>

## References

<references>

- https://docs.claude.com/en/docs/claude-code/skills
- https://platform.openai.com/docs/guides/agents-sdk
- https://openai.github.io/openai-agents-python/agents/
- https://langchain-ai.github.io/langgraph/tutorials/multi_agent/multi-agent-collaboration/
</references>
