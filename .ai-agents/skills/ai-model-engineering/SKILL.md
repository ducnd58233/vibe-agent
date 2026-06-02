---
name: ai-model-engineering
description: >-
  Designs, builds, adapts, evaluates, documents, serves, and monitors AI/ML model systems across computer vision, NLP/LLMs, speech/audio, recommender/ranking, tabular, multimodal, and generative AI. Use when implementing model training, fine-tuning, inference, evaluation, model cards, dataset cards, model monitoring, or AI product/model quality workflows; pair with MLOps for pipelines, registries, CI/CD/CT, and production operations.
disable-model-invocation: true
---

# AI Model Engineering

## What

Engineer AI/ML model systems from problem framing through data, baselines, training/adaptation, evaluation, documentation, serving, and monitoring.

## Why

AI systems fail when teams optimize model code while ignoring data quality, baselines, leakage, reproducibility, slice performance, safety, latency, cost, and post-deploy drift. This skill keeps model work evidence-based and production-aware.

## How

1. **Load context**
   - Inspect manifests, notebooks, data/model docs, train/eval scripts, inference code, configs, and monitoring docs.
   - Open [`../../stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md); load [`ai-modeling-multimodal.md`](../../stack-profiles/ai-modeling-multimodal.md), [`datascience.md`](../../stack-profiles/datascience.md), [`mlops.md`](../../stack-profiles/mlops.md), and runtime profiles that apply.
   - Open [`../../references/ai-model-development-patterns.md`](../../references/ai-model-development-patterns.md).
2. **Frame the task**
   - Define task type, user impact, decision boundary, non-ML baseline, target metric, guardrail metrics, latency/cost budget, and acceptable failure behavior.
   - Confirm whether an API/pretrained model, retrieval/rules, fine-tune/adapter, or training-from-scratch is justified.
3. **Audit data**
   - Check source, license/consent, splits, leakage, missingness, imbalance, label quality, PII/sensitive data, dataset version, and deployment representativeness.
   - Require dataset cards or equivalent notes for reusable datasets.
4. **Build baselines and experiments**
   - Establish simple heuristic/classical/pretrained baselines before custom complexity.
   - Log code version, data version, config, seed, environment, metrics, artifacts, and resource use.
   - Keep notebooks exploratory; move production training/eval into deterministic scripts.
5. **Evaluate rigorously**
   - Select metrics for the task and domain.
   - Compare against baseline, add slice tests and failure examples, and keep a held-out test set.
   - For LLM/RAG/agent systems, evaluate retrieval/context separately from final answer behavior.
6. **Prepare for serving**
   - Define model signature, preprocessing/postprocessing, artifact format, runtime target, batch/online mode, fallback, rollback, and resource envelope.
   - Pair with [`mlops-lifecycle`](../mlops-lifecycle/SKILL.md) for registry, promotion, CI/CD/CT, canary/shadow/A-B, and retraining.
7. **Monitor and improve**
   - Track data quality, input drift, prediction/output drift, model performance labels when available, latency, errors, cost, and business guardrails.
   - Define owners, alert thresholds, triage playbook, retraining trigger, and retirement criteria.
8. **Document**
   - Produce experiment report, model card, dataset card, evaluation report, deployment notes, and unresolved risks.

## When

Use for model implementation, fine-tuning, training scripts, inference services, evaluations, model/dataset documentation, and monitoring design across AI domains.

Do not use for generic data analysis only; use [`mlops-lifecycle`](../mlops-lifecycle/SKILL.md) when the main work is pipeline/registry/deployment operations.

## Routing & discovery

- Pair with [`ai-research-methodology`](../ai-research-methodology/SKILL.md) when the task requires literature review, reproduction, or novel method comparison.
- Pair with [`research-with-citations`](../research-with-citations/SKILL.md) for current model/paper/tool research.
- Pair with [`source-driven-development`](../source-driven-development/SKILL.md) for version-sensitive framework APIs.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for sensitive data, privacy, model abuse, prompt injection, or unsafe outputs.
- Pair with [`observability-monitoring`](../observability-monitoring/SKILL.md) for dashboards, alerts, and telemetry implementation.
- Hand off production pipelines, model registry, serving infrastructure, CI/CD/CT, and drift monitoring to [`mlops-lifecycle`](../mlops-lifecycle/SKILL.md); this skill owns model design, training/adaptation, evaluation, and model/data documentation.

## Permissions & authority

- Tools: Read, Grep, Glob, Edit; Shell for repo-documented tests/evals/train dry-runs.
- Paths: model source, configs, evals, docs, manifests, notebooks, monitoring configs; do not read secrets or sensitive datasets unless explicitly authorized.
- Ask before launching expensive training, GPU/cloud jobs, dataset mutation, model publication, production deployment, or external data upload.

## Verification

- [ ] Task, baseline, metrics, and guardrails are explicit.
- [ ] Data source, split, leakage, license, and quality risks are documented.
- [ ] Experiment artifacts are reproducible.
- [ ] Evaluation includes baseline comparison and relevant slices.
- [ ] Serving plan includes latency/cost/resource and rollback constraints.
- [ ] Monitoring signals, owner, thresholds, and retraining/retirement criteria are defined.
- [ ] Model card and dataset card are updated when applicable.
