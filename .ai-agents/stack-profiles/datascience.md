# Stack profile: Data Science

## Scope

<routing>

Applies to data-science and ML workflows in consumer repositories, including analytics, modeling, notebook pipelines, and reproducibility.

## When to load

- Dataset analysis and model training tasks
- Notebook-heavy workflows
- Metrics, benchmark, or experiment reporting tasks
</routing>

## Detection

<context>

- `environment.yml`, `pyproject.toml`, or `requirements*.txt` includes data/ML libraries
- Notebook files exist
- `data/` or `models/` directories exist

## Framework and tooling

- pandas, NumPy, scikit-learn, PyTorch (as detected)
- Optional experiment tracking: MLflow or W&B

## Repo layout conventions

- Read dependency manifest and `README.md` first
- Read data/model documentation before proposing pipelines
- Treat reproducibility metadata (seeds, hashes, versions) as required output
</context>

## Commands

<procedure>

- `pytest`
- `ruff check .`
- `mypy .`
- project-specific train/eval scripts from `README.md`
</procedure>

## Boundaries

<required>

- No unsupported benchmark claims
- Require source + date + license for dataset claims
- Require held-out evaluation details for model performance claims
- Mark unverified metrics as `UNVERIFIED`
</required>

## References

<references>

- https://docs.conda.io/projects/conda/en/latest/user-guide/tasks/manage-environments.html
- https://scikit-learn.org/stable/user_guide.html
- https://pandas.pydata.org/docs/
- https://pytorch.org/docs/stable/index.html
- https://mlflow.org
- https://drivendata.github.io/cookiecutter-data-science/
</references>
