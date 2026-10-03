# Agent evaluation patterns

<context>

Use this reference when validating whether an agent, skill, command, or hook actually improves outcomes.
</context>

## Evaluation surfaces

<rules>

- **Static checks:** router consistency, schema/frontmatter validation, missing links, permission mismatch.
- **Golden prompts:** stable tasks with expected artifacts or acceptance criteria.
- **Adversarial prompts:** ambiguous requests, unsafe operations, secret-seeking, missing context, stale docs.
- **Forward tests:** run the asset in a fresh context with only task-local inputs.
- **Regression tests:** scriptable checks for hooks and generated files.
</rules>

## Minimal evaluation loop

<procedure>

1. Define the task and expected behavior.
2. Run the asset without leaking the expected answer.
3. Score output against a rubric.
4. Patch the asset, not the model, when failure is procedural.
5. Keep the smallest reusable test artifact that catches the issue.
</procedure>

## Rubric

<rules>

| Dimension | Pass signal |
|---|---|
| Routing | Correct asset selected from router |
| Context | Reads only relevant files and cited docs |
| Safety | Respects permissions and asks before risky actions |
| Correctness | Produces verifiable output |
| Efficiency | Avoids unnecessary fan-out and context bloat |
| Handoff | Output is actionable for the next command/persona |
</rules>

## Reliability across repeated runs

<rules>

A pass on one run says little about whether an asset is dependable. Measure repeated trials.

| Metric | Meaning | Use it for |
|---|---|---|
| `pass@k` | At least one of `k` trials succeeds | Tasks where one good answer is enough |
| `pass^k` | All `k` trials succeed | Anything that runs unattended or for many people |

Anthropic's evaluation guide gives the arithmetic: a 75% per-trial success rate over three trials passes all three about 42% of the time (0.75 cubed). It also lists three grader kinds: code-based (fast, objective, brittle to valid variations), model-based (flexible, non-deterministic, needs calibration), and human (the reference standard, slow). Source: [Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents).

How to apply it here:

1. Run the same task `k` times (three is a start) on a copy of the same input. Point any outward action at a test target.
2. Count the runs that pass **every** acceptance row. Report `passes / k` and the per-row failures.
3. Read the transcripts of the failures before changing anything. A broken grader and a broken agent look the same in a score.
4. Prefer a code-based grader. Use a model-based one only as a second opinion for a person, never as evidence for a gate: the runtime has no evidence source for model assertion.
5. Record the trial count before the run, and do not drop a failed trial afterwards.

A single-run result is stated as a single-run result.
</rules>

## References

<references>

- https://platform.openai.com/docs/guides/evals
- https://openai.github.io/openai-agents-python/tracing/
- https://docs.claude.com/en/docs/claude-code/sub-agents
</references>
