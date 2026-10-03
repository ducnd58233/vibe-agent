---
name: ai-research-methodology
description: >-
  Conducts AI/ML research workflows: literature review, paper triage, benchmark analysis, reproduction planning, experiment design, ablation studies, error analysis, research-to-engineering handoff, and evidence-backed recommendations across CV, NLP/LLMs, speech, multimodal, reinforcement learning, recommender systems, and generative AI. Use when understanding new AI methods, comparing models, designing experiments, or translating research into buildable model work.
disable-model-invocation: true
---

# AI Research Methodology

## How

<procedure>

1. **Scope the research question**
   - Define target task/domain, constraints, baseline, success metric, deployment context, and what decision the research must inform.
   - Separate "understand field" from "choose implementation" from "reproduce paper" from "design new experiment."
2. **Gather credible sources**
   - Prefer papers, official docs, benchmark pages, model cards, dataset cards, reproducible repos, and release notes.
   - Use [`research-with-citations`](../research-with-citations/SKILL.md) and [`source-driven-development`](../source-driven-development/SKILL.md) for current or version-sensitive claims.
3. **Triage evidence**
   - Record venue/date, code/data availability, license, compute requirements, evaluation setup, dataset overlap risks, and whether results are independent or self-reported.
   - Mark leaderboards and blog claims as `UNVERIFIED` unless methodology and test data are clear.
4. **Compare methods**
   - Compare baselines, data needs, architecture, training/adaptation approach, inference cost, latency, memory, robustness, safety, and maintainability.
   - Identify assumptions that may not hold in the consumer repo.
5. **Design reproduction or experiment**
   - Define minimal reproducible target: dataset subset, metric, expected range, hardware, time budget, seeds, configs, and stopping criteria.
   - Plan ablations for the core claim, not every paper detail.
   - Freeze splits, metric, thresholds, trial budget, and data terms before running; avoid test-set tuning and hidden leakage. Rules: [`research-integrity`](../../references/research-integrity.md).
6. **Analyze failures**
   - Inspect qualitative examples, slices, confusions, prompt/retrieval failures, modality-specific errors, and out-of-distribution behavior.
7. **Handoff to engineering**
   - Produce recommendation, selected method, rejected alternatives, implementation risks, data requirements, eval plan, monitoring implications, and open questions.
   - Pair with [`ai-model-engineering`](../ai-model-engineering/SKILL.md) for implementation.
</procedure>

## Routing & discovery

<routing>

- Pair with [`ai-model-engineering`](../ai-model-engineering/SKILL.md) for build/train/eval/monitor follow-through.
- Pair with [`evidence-based-analysis`](../evidence-based-analysis/SKILL.md) for final tradeoff recommendation.
- Pair with [`mlops-lifecycle`](../mlops-lifecycle/SKILL.md) when research affects production model lifecycle.
- Use [`../../references/ai-model-development-patterns.md`](../../references/ai-model-development-patterns.md) for lifecycle/evaluation/documentation checks.
- Use [`../../references/research-integrity.md`](../../references/research-integrity.md) for leakage, selection pressure, shortcutting, contamination, and data-terms rules.

Use before adopting new AI methods, model families, datasets, benchmarks, evaluation frameworks, or papers; use when a task needs research-grade evidence rather than direct implementation.
Do not use for ordinary coding or generic web research that is not AI/ML-method specific.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, WebSearch, WebFetch; Shell only for repo-documented reproduction/eval commands.
- Paths: papers, docs, eval artifacts, notebooks, model configs, experiment reports; no secrets or sensitive datasets without explicit authorization.
- Ask before downloading large models/datasets, using paid APIs, launching GPU/cloud jobs, or running long experiments.
</required>

## Verification

<verification>

- [ ] Research question and decision target are explicit.
- [ ] Sources are cited and source quality is graded.
- [ ] Claims distinguish paper-reported, independently reproduced, and unverified results.
- [ ] Reproduction/experiment plan has metric, data, compute, seed/config, and stop criteria.
- [ ] Splits, thresholds, and trial budget were fixed before any result; the held-out split is scored once; data licence and terms are recorded.
- [ ] Recommendation includes implementation, eval, safety, and monitoring implications.
</verification>
