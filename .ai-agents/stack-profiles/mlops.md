# Stack profile: MLOps

## Scope

<routing>

Applies to consumer repositories that train, evaluate, register, deploy, monitor, or retrain machine-learning models, including classical ML, deep learning, batch inference, online inference, feature pipelines, and model governance.

## When to load

- Editing ML training, evaluation, model registry, serving, inference, feature, or data pipeline code
- Adding CI/CD/CT for models, datasets, features, prompts, or evaluation suites
- Implementing model monitoring, drift detection, lineage, reproducibility, promotion, rollback, or A/B tests
- Integrating MLflow, Kubeflow, KServe, Airflow, Dagster, Prefect, DVC, Feast, or cloud ML platforms
</routing>

## Detection

<context>

- `mlflow`, `kubeflow`, `kserve`, `airflow`, `dagster`, `prefect`, `dvc`, `feast`, `bentoml`, `ray`, `sklearn`, `pytorch`, `tensorflow`
- Paths such as `models/`, `notebooks/`, `pipelines/`, `features/`, `training/`, `inference/`, `data/`, `evals/`
- Model artifacts, experiment tracking configs, feature store configs, data validation, model registry, or serving manifests

## Framework and tooling

- Experiment tracking and model registry: MLflow or repo-pinned equivalent
- Pipeline orchestration: Kubeflow Pipelines, Airflow, Dagster, Prefect, or cloud-native ML pipelines
- Data/model versioning: DVC, lakehouse metadata, object storage versioning, model registry aliases/tags
- Serving: KServe, BentoML, TorchServe, Triton, FastAPI/Axum/Go service wrappers, or cloud serving
- Monitoring: input/output drift, data quality, model quality, latency, errors, resource usage, and business KPIs

## Repo layout conventions

- Read manifests, pipeline definitions, data contracts, evaluation reports, model registry conventions, and serving manifests first
- Version code, data snapshot/contract, features, model artifact, training config, and evaluation result together
- Keep notebooks exploratory; production training/evaluation should be scripts/pipelines with deterministic configs
- Promote models through dev/staging/prod with explicit gates and rollback to previous model version
- Keep online inference code thin and observable; keep heavy training/data work in pipelines
</context>

## Commands

<procedure>

- Use repo-documented ML commands first
- Typical examples: `pytest`, `python -m <train>`, `mlflow ui`, `dvc repro`, `dvc status`, `kubectl apply --dry-run=server`, `kfp` pipeline validation
- Never launch expensive training, cloud jobs, or production deployment without explicit approval
</procedure>

## Boundaries

<required>

- Do not promote a model without evaluation results, lineage, artifact version, and rollback path
- Do not couple notebooks directly to production jobs without extracting deterministic scripts
- Do not treat test accuracy alone as production readiness; include drift, calibration, latency, fairness/safety where applicable
- Do not log sensitive training data, prompts, labels, or model outputs without policy review

## Security / performance appendix

- Monitor data quality, training-serving skew, feature freshness, model drift, prediction distribution, latency, and cost
- Gate model deployment on automated evaluation and human approval for high-impact decisions
- Track model lineage from dataset/version to artifact to deployment
- Add shadow/canary/A-B rollout where model behavior risk is significant
</required>

## References

<references>

- https://docs.cloud.google.com/architecture/mlops-continuous-delivery-and-automation-pipelines-in-machine-learning
- https://mlflow.org/docs/latest/index.html
- https://www.mlflow.org/docs/latest/ml/tracking
- https://www.mlflow.org/docs/latest/ml/model-registry/workflow/
- https://www.kubeflow.org/docs/
</references>
