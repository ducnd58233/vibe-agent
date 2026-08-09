# AI model development patterns

<context>

Use this reference when building, adapting, evaluating, documenting, or monitoring AI/ML models across CV, NLP, speech, recommender, tabular, multimodal, generative AI, and agentic AI tasks.
</context>

## Expert operating principles

<rules>

- **Start with the product task, not the model.** Define user need, decision boundary, latency/cost budget, failure tolerance, and acceptance metrics before selecting a model.
- **Use the simplest useful baseline.** Compare heuristics, retrieval, rules, pretrained APIs, classical ML, fine-tuning, and training-from-scratch before choosing complexity.
- **Treat data as the primary artifact.** Track source, license, consent, labeling process, splits, leakage checks, schema, quality, bias risks, and dataset version.
- **Make experiments reproducible.** Version code, data snapshot, config, seed, environment, model artifact, metrics, and hardware/runtime notes.
- **Evaluate by slices and failure modes.** Report aggregate metrics plus slices for domain, language, class, geography, device, demographic/proxy risks, long-tail examples, and adversarial cases where relevant.
- **Document intended use and limits.** Maintain model cards and dataset cards for models/datasets that may be reused, shipped, or audited.
- **Monitor after deployment.** Track input drift, prediction drift, data quality, performance labels when available, latency, errors, cost, and business guardrails.
</rules>

## Lifecycle checklist

<verification>

1. **Task framing**
   - Define task type: classification, regression, ranking, retrieval, generation, detection, segmentation, ASR/TTS, embedding, recommendation, control, or agent workflow.
   - Define baseline, target metric, guardrail metrics, latency/cost budget, and acceptable failure behavior.
2. **Data and labels**
   - Inspect data source, ownership/license, sampling process, missingness, imbalance, leakage, PII/sensitive data, and annotation quality.
   - Create train/validation/test splits that reflect deployment conditions; keep the final test set held out.
3. **Baseline**
   - Establish heuristic/classical/pretrained baseline before custom training.
   - Save baseline metrics and example failures.
4. **Model selection**
   - Choose between API model, open-weight model, fine-tune/adapter, classical ML, deep model, or custom architecture based on evidence.
   - For LLM/RAG/agent systems, treat prompt, retrieval, tools, memory, and eval set as model-system components.
5. **Training or adaptation**
   - Use deterministic configs where possible; log hyperparameters, seeds, framework versions, data versions, and resource use.
   - Track checkpoints and stop criteria; avoid tuning against the test set.
6. **Evaluation**
   - Use task-appropriate metrics: accuracy/F1/AUROC/AUPRC, calibration, mAP/IoU, WER/CER, BLEU/ROUGE/BERTScore, retrieval recall/precision/MRR/nDCG, human preference, cost/latency.
   - Add qualitative error analysis with representative examples.
7. **Safety and responsible AI**
   - Assess privacy, copyright/license, bias/fairness, misuse, harmful outputs, security abuse, and human oversight requirements.
   - Add refusal/escalation/human-review behavior for high-impact decisions.
8. **Packaging and serving**
   - Define model signature, preprocessing/postprocessing, artifact format, dependency versions, runtime target, batch/online mode, and rollback target.
9. **Monitoring**
   - Define reference data, monitoring frequency, alert thresholds, owner, triage playbook, retraining trigger, and retirement criteria.
10. **Documentation**
   - Produce or update experiment report, model card, dataset card, evaluation report, deployment notes, and unresolved risks.
</verification>

## Domain-specific cues

<rules>

| Area | Watch for | Typical metrics |
|---|---|---|
| Computer vision | data augmentation, label noise, camera/domain shift, class imbalance, privacy in images/video | accuracy/F1, mAP, IoU, Dice, latency/FPS |
| NLP / LLM | hallucination, grounding, prompt sensitivity, multilingual slices, toxicity/safety, context recall | exact match, F1, ROUGE/BLEU, pairwise preference, faithfulness, retrieval recall |
| Speech / audio | sampling rate, noise, accents, speaker overlap, streaming latency, privacy | WER/CER, MOS, latency, diarization error |
| Recommendations / ranking | delayed feedback, position bias, exploration, cold start, online/offline metric mismatch | nDCG, MRR, recall@k, CTR/conversion, calibration |
| Tabular / forecasting | leakage, temporal splits, feature freshness, calibration, explainability | RMSE/MAE, AUROC/AUPRC, calibration, business KPI |
| Multimodal | modality alignment, missing modalities, OCR/ASR errors, cross-modal grounding | task metric plus modality-specific error slices |

## Model and dataset cards

Include when a model/dataset can be reused, deployed, or audited:

- model/dataset name and version
- intended use and out-of-scope use
- training/evaluation data and license
- preprocessing and labeling process
- metrics, slices, confidence intervals if available
- limitations, bias/safety/privacy risks
- operational constraints: latency, hardware, cost, monitoring, rollback
- owner, date, and update policy
</rules>

## Related references

<references>

- [`agent-evaluation-patterns.md`](agent-evaluation-patterns.md)
- [`context-management-patterns.md`](context-management-patterns.md)
- [`ci-cd-observability-patterns.md`](ci-cd-observability-patterns.md)
- [`security-checklist.md`](security-checklist.md)
</references>

## Source anchors

<rules>

- Google Cloud MLOps guidance emphasizes automation and monitoring across ML construction, including integration, testing, release, deployment, and infrastructure management.
- Google Rules of ML recommends treating most early ML work as engineering: solid pipelines and features before complex algorithms.
- Hugging Face model cards describe model intent, limitations, training parameters, datasets, and evaluation results.
- Hugging Face dataset cards document dataset contents, context, creation process, and bias considerations.
- OpenAI evaluation guidance recommends continuous evaluation and growing eval sets over time for nondeterministic AI systems.
- Azure ML monitoring guidance highlights data drift, prediction drift, data quality, feature attribution drift, model performance, reference data, thresholds, and alert frequency.
</rules>
