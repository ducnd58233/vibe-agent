---
name: mlops-lifecycle
description: >-
  Designs and implements MLOps workflows for experiment tracking, data/model versioning, evaluation gates, registries, CI/CD/continuous training, serving, monitoring, drift detection, rollback, and governance. Use when working on ML pipelines, model deployment, MLflow/Kubeflow, feature pipelines, inference services, or production model lifecycle.
disable-model-invocation: true
---

# MLOps Lifecycle

## What

Operate machine-learning systems across the lifecycle: data, features, experiments, training, evaluation, registry, serving, monitoring, retraining, rollback, and governance.

## Why

ML systems change when code, data, features, configs, model artifacts, prompts, or traffic distributions change. Production readiness requires lineage, reproducibility, evaluation gates, monitoring, and safe promotion.

## How

1. **Load stack context**
   - Inspect ML manifests, notebooks, pipeline definitions, model registry conventions, serving code, evals, and data contracts.
   - Open [`mlops.md`](../../stack-profiles/mlops.md) and compose app/runtime profiles.
2. **Identify lifecycle stage**
   - Exploration, training, evaluation, registry, deployment, serving, monitoring, retraining, or retirement.
3. **Make work reproducible**
   - Version code, data snapshot/contract, features, config, artifact, metrics, and environment.
   - Move production logic out of notebooks into deterministic scripts/pipelines.
4. **Gate model promotion**
   - Require evaluation metrics, baseline comparison, slice tests, latency/cost checks, lineage, and rollback target.
   - Add human approval for high-impact or regulated decisions.
5. **Serve safely**
   - Keep inference APIs observable, bounded, and rollbackable.
   - Add canary/shadow/A-B rollout where model behavior risk is material.
6. **Monitor after deploy**
   - Track input drift, output distribution, data quality, feature freshness, latency, errors, cost, business KPIs, and feedback labels.
   - Define retraining triggers and model retirement criteria.

## When

Use for ML pipelines, model lifecycle, MLflow/Kubeflow work, inference services, and production model monitoring. Do not use for generic data analysis unless it affects production ML lifecycle.

## Routing & discovery

- Pair with [`test-driven-development`](../test-driven-development/SKILL.md) for evaluation and regression gates.
- Pair with [`observability-monitoring`](../observability-monitoring/SKILL.md) for model/service monitoring.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for sensitive data, model abuse, and governance.
- Defer model architecture, training/fine-tuning, and evaluation design to [`ai-model-engineering`](../ai-model-engineering/SKILL.md); this skill owns pipelines, registries, serving infrastructure, CI/CD/CT, drift detection, and rollout/rollback.

## Permissions & authority

- Tools: Read, Grep, Glob, Edit; Shell for repo-documented tests/evals.
- Paths: ML source, configs, evals, pipelines, manifests, docs; no sensitive datasets or secrets unless authorized.
- Ask before launching expensive training, cloud jobs, production deployment, or data-mutating pipelines.

## Verification

- [ ] Code, data, features, config, artifact, metrics, and environment lineage are tracked.
- [ ] Promotion gate compares against baseline and includes slice/edge-case evaluation.
- [ ] Serving path has latency/error/resource monitoring and rollback target.
- [ ] Drift/data-quality/model-quality monitoring is defined or explicitly deferred.
- [ ] Sensitive data/model-output logging is avoided or policy-reviewed.

## References

- https://docs.cloud.google.com/architecture/mlops-continuous-delivery-and-automation-pipelines-in-machine-learning
- https://mlflow.org/docs/latest/index.html
- https://www.mlflow.org/docs/latest/ml/tracking
- https://www.mlflow.org/docs/latest/ml/model-registry/workflow/
- https://www.kubeflow.org/docs/
