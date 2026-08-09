# Stack profile: AI model engineering and multimodal modeling

## Scope

<routing>

Applies to consumer repositories that build, adapt, train, evaluate, serve, or monitor AI/ML model systems across computer vision, NLP/LLMs, speech/audio, recommender/ranking, tabular, multimodal, and generative AI tasks.

## When to load

- Implementing or reviewing model training, fine-tuning, adapter, embedding, retrieval, inference, or evaluation code
- Working with CV, NLP/LLM, speech/audio, multimodal, recommender, ranking, tabular ML, or generative AI systems
- Creating model cards, dataset cards, experiment reports, eval reports, or monitoring plans
- Selecting between API models, open-weight models, classical ML, pretrained models, fine-tuning, or custom training
</routing>

## Detection

<context>

- Manifests mention `torch`, `tensorflow`, `jax`, `sklearn`, `transformers`, `datasets`, `tokenizers`, `sentence-transformers`, `openai`, `anthropic`, `langchain`, `llama-index`, `opencv`, `ultralytics`, `spacy`, `nltk`, `librosa`, `whisper`, `torchaudio`, `xgboost`, `lightgbm`, `catboost`, `mlflow`, `wandb`, `dvc`
- Paths such as `models/`, `training/`, `inference/`, `evals/`, `notebooks/`, `datasets/`, `features/`, `prompts/`, `embeddings/`, `rag/`, `vision/`, `speech/`
- Files such as model configs, checkpoints, tokenizer configs, prompt/eval datasets, model cards, dataset cards, experiment logs

## Framework and tooling

- Core ML: scikit-learn, PyTorch, TensorFlow/Keras, JAX as detected
- LLM/NLP: Transformers, tokenizers, embeddings, RAG pipelines, vector stores, prompt/eval tooling as detected
- CV: OpenCV, torchvision, detection/segmentation libraries, image/video preprocessing
- Speech/audio: librosa, torchaudio, Whisper-like ASR, TTS, diarization tools
- Experiment tracking: MLflow, Weights & Biases, TensorBoard, DVC, or repo-pinned equivalent
- Serving: FastAPI/Axum/Go wrappers, BentoML, KServe, TorchServe, Triton, cloud/model API providers, or batch pipelines

## Repo layout conventions

- Read manifests, README, data/model docs, experiment configs, train/eval scripts, and inference wrappers before editing
- Keep exploratory notebooks separate from production training/evaluation scripts
- Keep preprocessing/postprocessing versioned with the model artifact
- Store metrics and model/dataset documentation near artifacts or in repo-documented report paths
- Treat prompts, retrieval config, model parameters, and eval sets as versioned AI system artifacts
</context>

## Commands

<procedure>

- Use repo-documented commands first
- Typical examples: `pytest`, `python -m <package>.train --dry-run`, `python -m <package>.evaluate`, `mlflow ui`, `dvc status`, `dvc repro`, `ruff check .`, `mypy .`
- Never launch expensive training, large downloads, paid API evals, GPU/cloud jobs, or publication without explicit approval
</procedure>

## Boundaries

<required>

- Do not claim model quality without evaluation data, baseline comparison, and split details
- Do not tune against the final test set
- Do not use datasets without license/source/consent review
- Do not log secrets, PII, sensitive prompts, labels, or model outputs without policy review
- Do not publish models/datasets or upload artifacts externally without approval
- Do not treat benchmark leaderboard wins as product readiness without task-specific evals

## Security / performance appendix

- Track data leakage, prompt injection, training data extraction, memorization, unsafe outputs, and model abuse risks where relevant
- Include latency, throughput, memory, accelerator needs, cost per inference/training run, and carbon/energy notes when material
- Monitor input drift, output/prediction drift, data quality, model quality labels, feature freshness, latency, errors, cost, and business guardrails
</required>

## References

<references>

- https://developers.google.com/machine-learning/guides/rules-of-ml/
- https://docs.cloud.google.com/architecture/mlops-continuous-delivery-and-automation-pipelines-in-machine-learning
- https://huggingface.co/docs/hub/model-cards
- https://huggingface.co/docs/datasets/dataset_card
- https://developers.openai.com/api/docs/guides/evaluation-best-practices
- https://learn.microsoft.com/en-us/azure/machine-learning/concept-model-monitoring
</references>
