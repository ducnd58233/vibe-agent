# Agent evaluation patterns

Use this reference when validating whether an agent, skill, command, or hook actually improves outcomes.

## Evaluation surfaces

- **Static checks:** router consistency, schema/frontmatter validation, missing links, permission mismatch.
- **Golden prompts:** stable tasks with expected artifacts or acceptance criteria.
- **Adversarial prompts:** ambiguous requests, unsafe operations, secret-seeking, missing context, stale docs.
- **Forward tests:** run the asset in a fresh context with only task-local inputs.
- **Regression tests:** scriptable checks for hooks and generated files.

## Minimal evaluation loop

1. Define the task and expected behavior.
2. Run the asset without leaking the expected answer.
3. Score output against a rubric.
4. Patch the asset, not the model, when failure is procedural.
5. Keep the smallest reusable test artifact that catches the issue.

## Rubric

| Dimension | Pass signal |
|---|---|
| Routing | Correct asset selected from router |
| Context | Reads only relevant files and cited docs |
| Safety | Respects permissions and asks before risky actions |
| Correctness | Produces verifiable output |
| Efficiency | Avoids unnecessary fan-out and context bloat |
| Handoff | Output is actionable for the next command/persona |

## References

- https://platform.openai.com/docs/guides/evals
- https://openai.github.io/openai-agents-python/tracing/
- https://docs.claude.com/en/docs/claude-code/sub-agents
