# Context management patterns

<context>

Use this reference when designing assets that need many files, long docs, research, or multi-step execution.
</context>

## Patterns

<rules>

- **Router first:** load the family router, then only matching assets.
- **Manifest scan before deep read:** inspect manifests and directory names before opening large files.
- **Digest large reads:** convert broad exploration into short evidence summaries.
- **Progressive references:** keep `SKILL.md` lean and link detailed references by condition.
- **Checkpoint sequential work:** `/spec` → `/plan` → `/build` should preserve artifacts rather than relying on memory.
</rules>

## Anti-patterns

<antipatterns>

- Always-on instructions that restate every skill.
- Agent prompts that embed whole stack guides instead of linking stack profiles.
- Parallel agents reading unrelated context because scope was not constrained.
- Research reports without source quality or conflict handling.
</antipatterns>

## Context budget checklist

<verification>

- [ ] The asset says what to read first.
- [ ] Large optional references are loaded only when relevant.
- [ ] The output includes a compact handoff summary.
- [ ] Unverified or stale claims are labeled.
- [ ] The next step can continue from an artifact path, not chat memory only.
</verification>

## References

<references>

- https://docs.claude.com/en/docs/claude-code/skills
- https://platform.openai.com/docs/guides/agents-sdk
- https://langchain-ai.github.io/langgraph/concepts/multi_agent/
</references>
