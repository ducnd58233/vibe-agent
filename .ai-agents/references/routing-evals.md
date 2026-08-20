# Routing evals (intent → expected asset)

<context>

Lightweight, deterministic fixtures for the toolkit's routing layer. Each row pairs a
representative **user intent** with the asset the router should select.

`vibe-agent doctor` validates this table (`runtime/cmd/routing_evals.go`, tested in the
suite that gates the build). Five rules, each failing:

1. Every row has three columns.
2. The **Expected asset** cell holds exactly one link, and it resolves on disk.
3. The **Expected family** matches the folder the target actually lives in. A row saying
   `command` while pointing into `skills/` describes a route nobody can take, and the
   link check alone cannot see it.
4. No two rows claim the same intent, since the second would silently redefine the first.
5. No intent contains its own asset's slug. A fixture that spells out its answer tests
   string matching rather than routing. Only hyphenated slugs count as evidence - a
   one-word slug like `review` is also the plain verb a person would use.

Coverage is **reported, never enforced**: the maintenance rule below asks for a row when
users will ask for an asset by intent, and that judgment is not something a check can
make. `doctor` prints the ratio so it stays visible.

Those checks are deterministic: they verify the target exists, is filed where it says, and
stays named as documented. They cannot say whether the router *actually* selects the
asset. `vibe-agent eval routing` answers that separately - see **Model-graded run** below.
For skill and agent behavioral evaluation more broadly, see
[`agent-evaluation-patterns.md`](agent-evaluation-patterns.md).
</context>

## Fixtures

<references>

| User intent | Expected family | Expected asset |
|-------------|-----------------|----------------|
| Write a failing test first, then implement | skill | [`test-driven-development`](../skills/test-driven-development/SKILL.md) |
| Review a diff across five axes before merge | command | [`review.md`](../commands/review.md) |
| Research a topic with verifiable citations | skill | [`research-with-citations`](../skills/research-with-citations/SKILL.md) |
| Choose between options from gathered evidence | skill | [`evidence-based-analysis`](../skills/evidence-based-analysis/SKILL.md) |
| Ground version-sensitive framework APIs in official docs | skill | [`source-driven-development`](../skills/source-driven-development/SKILL.md) |
| Curate which files/retrieval load into the window | skill | [`context-engineering`](../skills/context-engineering/SKILL.md) |
| Keep output concise in a high-volume loop | skill | [`token-efficient-execution`](../skills/token-efficient-execution/SKILL.md) |
| Design CI/CD, IaC, deploy gates, rollback | skill | [`devops-platform-delivery`](../skills/devops-platform-delivery/SKILL.md) |
| Add OpenTelemetry traces, metrics, SLOs, alerts | skill | [`observability-monitoring`](../skills/observability-monitoring/SKILL.md) |
| Train/fine-tune and evaluate a model | skill | [`ai-model-engineering`](../skills/ai-model-engineering/SKILL.md) |
| Build ML pipelines, registry, drift monitoring | skill | [`mlops-lifecycle`](../skills/mlops-lifecycle/SKILL.md) |
| Plan lifecycle metrics, feature flags, deprecation | skill | [`product-lifecycle-management`](../skills/product-lifecycle-management/SKILL.md) |
| Staged rollout and rollback for a risky release | skill | [`shipping-and-launch`](../skills/shipping-and-launch/SKILL.md) |
| Shape a vague feature idea into scoped options | skill | [`idea-refine`](../skills/idea-refine/SKILL.md) |
| Optimize a slow SQL/NoSQL query or explain plan | skill | [`database-query-optimization`](../skills/database-query-optimization/SKILL.md) |
| Latency regressed on an API path and the client feels janky | skill | [`performance-optimization`](../skills/performance-optimization/SKILL.md) |
| Production errors spiked and the behavior is unexplained | skill | [`debugging-and-error-recovery`](../skills/debugging-and-error-recovery/SKILL.md) |
| Define a REST contract and shared types across frontend and backend | skill | [`api-and-interface-design`](../skills/api-and-interface-design/SKILL.md) |
| Record a decision affecting several modules so it is not relitigated | skill | [`documentation-and-adrs`](../skills/documentation-and-adrs/SKILL.md) |
| Settle branching habits and untangle a messy merge | skill | [`git-workflow-and-versioning`](../skills/git-workflow-and-versioning/SKILL.md) |
| Run the citation digest slash command on a scoped topic | command | [`research.md`](../commands/research.md) |
| Audit toolkit asset health (routers, links, hooks) | command | [`doctor.md`](../commands/doctor.md) |
| Harden toolkit permissions, hooks, tool boundaries | command | [`harden.md`](../commands/harden.md) |
| Parallel evidence question with merged verdict | command | [`investigate.md`](../commands/investigate.md) |
| Implement the next task on its own branch and commit it | command | [`build.md`](../commands/build.md) |
| Run the pre-ship specialist fan-out for a GO/NO-GO | command | [`ship.md`](../commands/ship.md) |
| Security review: auth, injection, secrets, LLM tool surface | agent | [`security-auditor.md`](../agents/security-auditor.md) |
| Isolated five-axis code review persona | agent | [`code-reviewer.md`](../agents/code-reviewer.md) |
| Audit the agent/skill system itself | agent | [`agent-systems-auditor.md`](../agents/agent-systems-auditor.md) |
| Hand coverage gaps to an isolated testing specialist | agent | [`test-engineer.md`](../agents/test-engineer.md) |
| Serve a trained model behind an inference API with monitoring | agent | [`ai-engineer.md`](../agents/ai-engineer.md) |
| Triage recent papers and turn a method into an experiment plan | agent | [`ai-researcher.md`](../agents/ai-researcher.md) |
| Design module boundaries and data flow before a large feature | agent | [`architect-planner.md`](../agents/architect-planner.md) |
| Isolated persona to compare options and report tradeoffs with citations | agent | [`data-analyst.md`](../agents/data-analyst.md) |
| Isolated persona for index choice, explain plans, and lock contention | agent | [`database-query-auditor.md`](../agents/database-query-auditor.md) |
| Review pipeline, infra, alerting, and launch readiness risks | agent | [`devops-sre-auditor.md`](../agents/devops-sre-auditor.md) |
| Check UI fidelity, accessibility, and token usage against the handoff | agent | [`product-design-reviewer.md`](../agents/product-design-reviewer.md) |
| Write exploratory charters and a browser matrix for release signoff | agent | [`qa-tester.md`](../agents/qa-tester.md) |
| Isolated persona to gather and verify sources on a scoped topic | agent | [`research-investigator.md`](../agents/research-investigator.md) |
| Validate the citations in a finished report and flag unsupported claims | agent | [`source-auditor.md`](../agents/source-auditor.md) |
| Turn collected evidence into a cited comparison table | command | [`analyze.md`](../commands/analyze.md) |
| Run the slash command that reduces complexity with tests still green | command | [`code-simplify.md`](../commands/code-simplify.md) |
| Build UI registry-first with render evidence and anti-slop gates | command | [`design.md`](../commands/design.md) |
| Drive one objective end to end through the delivery pipeline | command | [`goal.md`](../commands/goal.md) |
| Turn an approved specification into ordered tasks with acceptance criteria | command | [`plan.md`](../commands/plan.md) |
| Write the structured work definition before any implementation | command | [`spec.md`](../commands/spec.md) |
| Start from a failing case, or run focused proof for a fix | command | [`test.md`](../commands/test.md) |
| Design the guides, sensors, and boundaries around a coding agent | skill | [`agent-harness-engineering`](../skills/agent-harness-engineering/SKILL.md) |
| Plan a reproduction and design ablations from a paper | skill | [`ai-research-methodology`](../skills/ai-research-methodology/SKILL.md) |
| Structure services, queues, and transactional boundaries on the server | skill | [`backend-engineering`](../skills/backend-engineering/SKILL.md) |
| Drive a real page and read console and network while verifying | skill | [`browser-testing-with-devtools`](../skills/browser-testing-with-devtools/SKILL.md) |
| What to look for when judging a change beyond correctness | skill | [`code-review-and-quality`](../skills/code-review-and-quality/SKILL.md) |
| Principles for reducing nesting and duplication while refactoring | skill | [`code-simplification`](../skills/code-simplification/SKILL.md) |
| Handle races, backpressure, and socket fan-out safely | skill | [`concurrency-realtime-systems`](../skills/concurrency-realtime-systems/SKILL.md) |
| Apply SOLID, DRY, KISS, and YAGNI to a module boundary | skill | [`engineering-principles`](../skills/engineering-principles/SKILL.md) |
| Build accessible components, state, and rendering in the browser | skill | [`frontend-ui-engineering`](../skills/frontend-ui-engineering/SKILL.md) |
| The rules behind driving one outcome across many sessions | skill | [`goal-driven-delivery`](../skills/goal-driven-delivery/SKILL.md) |
| Check assumptions and keep the diff surgical on a risky change | skill | [`karpathy-guardrails`](../skills/karpathy-guardrails/SKILL.md) |
| Split a large change into ordered, verifiable pieces | skill | [`planning-and-task-breakdown`](../skills/planning-and-task-breakdown/SKILL.md) |
| Set up tokens, components, and a registry for a shared visual language | skill | [`product-design-systems`](../skills/product-design-systems/SKILL.md) |
| Decide coverage across manual, automation, and regression layers | skill | [`qa-testing-strategy`](../skills/qa-testing-strategy/SKILL.md) |
| Keep tokens out of bundles, logs, storage, and responses while writing | skill | [`secure-by-default`](../skills/secure-by-default/SKILL.md) |
| Threat-model an existing surface and close authorization holes | skill | [`security-and-hardening`](../skills/security-and-hardening/SKILL.md) |
| Why the written definition comes before code, and what it holds | skill | [`spec-driven-development`](../skills/spec-driven-development/SKILL.md) |
| Manage services, users, cron, and disk on a Linux host | skill | [`system-administration-ops`](../skills/system-administration-ops/SKILL.md) |
| Match an implemented screen to the handoff pixel for pixel | skill | [`ui-design-fidelity`](../skills/ui-design-fidelity/SKILL.md) |
| Figure out which asset to load for the work in front of me | skill | [`using-agent-skills`](../skills/using-agent-skills/SKILL.md) |
</references>

## Model-graded run

<procedure>

Run the fixtures against a live coding-agent runner when router wording changes:

```bash
vibe-agent eval routing --trials 1 --jobs 6 --timeout 90s
```

The default runner is `codex`. Compare hosts by repeating `--runner` or by
comma-separating presets:

```bash
vibe-agent eval routing --runner codex --runner claude
vibe-agent eval routing --runner codex,claude
```

Available presets:

| Preset | Command | Notes |
|---|---|---|
| `codex` | `codex exec --ephemeral --sandbox read-only --json -` | Non-interactive mode; reads stdin. |
| `claude` | `claude -p` | Reads stdin. |
| `cursor` | `cursor-agent --print --mode ask --trust` | Passes the prompt as an argument. |
| `opencode` | `opencode run` | Passes the prompt as an argument. |
| `all` | every preset above | Useful for local comparison, noisy if a host is not authenticated. |

Use a raw command for any other host:

```bash
vibe-agent eval routing --runner "claude -p"
```

The runner must either read the prompt from stdin or be one of the argument-style
presets above. Codex JSONL output is accepted; the eval grades the final
`agent_message`.
</procedure>

<references>

OpenAI Docs: Codex non-interactive mode is `codex exec`; the CLI reference says
`PROMPT` can be `-` to read stdin.
</references>

## Maintenance

<rules>

- Add a row when you add a routable asset users will ask for by intent.
- When you rename or move an asset, update its row in the same change; `doctor` fails on
  a dangling **Expected asset** link and on a family that no longer matches its folder.
- Keep intents phrased as a user would ask, not as the asset name.
- Boundary-sensitive pairs are intentionally pinned here (model-engineering vs MLOps,
  context-engineering vs token-efficient-execution, product-lifecycle vs shipping/idea)
  so seam drift surfaces as a failing route.
</rules>
