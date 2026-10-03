# Research integrity

<context>

Owner of the rules that keep an experiment or a piece of research from reporting a result it did not earn. Read it before designing, running, or writing up any experiment, benchmark, ablation, or model evaluation, and before citing a number from one.

These failures share one property: **nothing crashes and every number looks like success.** A leaked split gives a high score. A loop that retries until a threshold passes gives a passing score. An edited grader gives a perfect score. A person reading the final table sees a good result either way, which is why the rules below are written as mechanical facts to record and check, not as advice to be careful.

The toolkit's own loop makes the second failure easy. On `researcher-delivery`, a missed threshold at `results_eval` routes back to `hypothesis` with no human step, so an agent that keeps trying until the number clears is doing selection against whatever split `results_eval` reads. The guards below exist because that loop is automatic.
</context>

## Failure classes

<rules>

| Class | What it looks like | Signature to look for |
|---|---|---|
| **Leakage** | Preprocessing (scaling, imputation, feature selection, tokenizer or vocabulary fitting) fit on all data before the split. Duplicates or near-duplicates across splits. The same user, patient, session, or document in train and test. A feature built from the label or from the future. Random split of time-ordered data. | Validation very high, test low. Or test suspiciously above validation. A feature importance dominated by an ID, timestamp, or filename. Score collapses under a group or time split. |
| **Selection pressure** | Many configurations tried, best one reported. Seed picked after seeing results. Metric, split, or filter changed after the first look. Hard examples or outliers dropped. Retrying until a threshold passes. | `trials` is large and unreported. Variance across seeds ignored. The reported split was scored many times. |
| **Shortcutting by the agent** | Editing the evaluator, a threshold, a test, or a label. Reading the answer key. Hardcoding expected outputs. A `try/except` that skips samples that fail and averages the rest. Monkeypatching the metric. Disabling the timer. | Evaluated count below total count. A diff touching eval code or thresholds in the same change as a result. A perfect or impossible score. |
| **Contamination and judging** | The benchmark was in the model's pretraining data. Prompts or few-shot examples tuned on eval items. An LLM judge from the same family as the model it scores. A hosted model whose version changed mid-study. | Score far above what the task's difficulty supports. Judge and subject share a provider. No model version or date recorded. |
| **Fabrication** | A number, citation, or result written before it was run or without a source. Placeholder text left in a report. | A figure with no log or file behind it. A URL that does not resolve, or resolves to a page that does not say the claim. |
| **Selective reporting** | Failed and abandoned runs omitted. A baseline given less tuning than the method. Baseline not re-run under the changed setting. Single-seed differences presented as findings. | Findings cite one run. Baseline numbers copied from a paper while the method was tuned on local data. |
| **Data terms** | A dataset used outside its licence. Scraped content that a site's terms or `robots.txt` forbid for this use. Personal data without a lawful basis. Model-provider output reused as training or evaluation data in a way the provider's terms restrict. | No source, licence, or terms recorded for a dataset. Output of a hosted model flowing into another model's training set. |

Leakage has a published taxonomy of eight types and a model-info-sheet remedy, and in the authors' survey affected papers in 17 fields; correcting leakage in their civil-war case study removed the claimed advantage of complex ML models over older methods (Kapoor and Narayanan, Patterns 2023, see Sources). Agent shortcutting is documented in frontier models: METR reports models changing tests or scoring code, reading the stored answer, or disabling timing, and found it far more common where the model could see the whole scoring function. An independent evaluation of an autonomous research system reported experiments that failed to re-run baselines, producing misleading comparisons, and some manuscripts with hallucinated numbers (Beel et al., see Sources).
</rules>

## Before the run: freeze the protocol

<required>

Write these in the PLAN under **Evaluation protocol** and **Data and terms**, and get them approved, before any result exists. After the run they are read-only. If one turns out wrong, record that and ask a person; do not edit it so the run passes.

1. **Splits.** Name train, selection (validation or cross-validation), and held-out. Split by the unit that must not cross (user, patient, session, document, time), not by row. Fit every preprocessing step on train only. State how duplicates and near-duplicates were removed across splits.
2. **What is tuned where.** Hyperparameters, prompts, features, checkpoints, early stopping, and thresholds are chosen on the selection split only. The held-out split is scored **once**, at the end.
3. **Metric and thresholds.** The metric, its direction, the thresholds, and `maxGap` (the largest accepted difference between the selection and held-out numbers, per metric) are fixed here. There is no universal `maxGap`; the author sets it from the task's noise and states why.
4. **Trial budget.** The number of configurations the project may try. Count failures and abandoned runs.
5. **Baselines.** Each baseline gets the same tuning budget and the same splits as the method, and is re-run, not copied, whenever the setting differs from the source paper's.
6. **Variance.** Seeds or resamples enough to report spread, and the test used to compare (a paired test or a bootstrap interval, chosen for the metric; see Dror et al.). A difference inside the spread is not a finding.
7. **Data and terms.** For each dataset and each hosted model: source, licence, terms of use, crawl or `robots.txt` restrictions where it was scraped, consent and personal data, and any provider term limiting reuse of outputs. An unknown is written as unknown and goes to a person.
8. **LLM and agent evaluations.** Record model name, exact version or date, decoding settings, and prompt. Name the contamination check (a benchmark released after the training cutoff, a private or held-back set, or an overlap search) or state that none was done. Use a judge from a different family than the subject, or state the risk.
</required>

## During and after the run

<required>

- **Never edit the evaluator.** Do not change metric code, a grader, a threshold, labels, the eval set, or a test in the same change that produces a result, to make it pass. A needed fix to the evaluator is its own change, made before the run is scored and recorded in the PLAN.
- **Never read or derive from held-out labels.** Not to pick features, filter data, tune a prompt, or sanity-check a guess.
- **Report coverage.** If an evaluation skips samples (errors, timeouts, refusals), report evaluated over total, and count a skipped sample as a failure unless the PLAN said otherwise.
- **No quiet repair.** If leakage or a contaminated split is found after a result exists, say so, mark every result from that split `INVALID`, and treat the held-out split as spent. Do not fix the leak and re-score the same split as if nothing happened.
- **One held-out look.** A held-out split that has been scored more than once is a selection split now. Report on fresh data, or call the result exploratory.
- **Record the truth in `METRICS.json`.** Write the `integrity` block ([`experiment.md`](../commands/experiment.md) "STATUS.md contract"). `reportedSplitEvaluations` comes from counting entries in the experiment ledger that scored that split, not from memory.
- **Findings state the cost of the number.** Trials tried, runs that failed, spread across seeds, coverage, the selection-versus-held-out gap, and which result is exploratory. A claim with only a favourable run behind it is `UNVERIFIED`.
- **No unsourced numbers.** A figure comes from a log or file in this run, or from a cited source you opened. Otherwise it is `UNVERIFIED`.
</required>

## What the guards check, and what they do not

<context>

| Guard | Where | Checks |
|---|---|---|
| Auto design gate | `vibe-agent auto gate` at `approve_design` | PLAN has non-empty **Evaluation protocol** and **Data and terms** sections. Auto mode otherwise skips the human design approval. |
| Results verifier | `results_eval` (`results` verifier) | `METRICS.json` has an `integrity` block. For `held_out_eval`: selection and reported splits differ; `trials` at least 1; reported split scored exactly once; each gated metric has a selection value and a declared `maxGap`; the absolute gap is within it. |
| Schema | [`experiment-run.schema.json`](../../schemas/experiment-run.schema.json) | Same rules for ledger files under `experiments/`. |

These read what the author wrote. They catch the failures nobody notices: a forgotten split, a test set reused across retries, a validation score that does not survive on test. They do **not** catch an author who writes false values, a leak that affects both splits equally, contamination of a pretrained model, or a bad `maxGap`. Those need the human design gate, a fresh look at the data, and the Data and terms review. A passing `results_eval` means the record is consistent, not that the science is sound.

A failure beginning `INTEGRITY:` is not a miss to retry. Looping to `hypothesis` re-scores the reported split. Fix the cause (re-split, a new held-out set, a corrected record) or stop and ask.
</context>

## Antipatterns

<antipatterns>

- "The test score is lower, so I will tune a little more and check again." That is tuning on test.
- "The threshold looks too strict, so I will lower it." The threshold was fixed before the run.
- "A few samples crash the metric, so I will skip them." Report coverage.
- "Validation is 0.97 and test is 0.85, but test passes the bar." The gap is the finding.
- "The paper reports 0.91, so our 0.88 is close." A copied baseline from a different setting is not a baseline.
- "The best of 30 runs is the result." Report the thirty.
- "The model said the tests pass." A model's statement is not evidence; a logged exit code or a file is.
</antipatterns>

## When to stop and ask a person

<escalation>

- A dataset's licence, terms of use, `robots.txt`, consent basis, or provider terms are unclear or restrictive for the planned use.
- Personal, health, financial, or otherwise sensitive records are in the data.
- An `INTEGRITY:` failure that correcting the record cannot fix.
- The held-out split is spent, or leakage is found after a result exists.
- The only way to meet the threshold is to change the evaluator, the split, or the threshold.
</escalation>

## Sources

<references>

Opened and read in the session that wrote this file:

- Anthropic Commercial Terms of Service, section D.4, effective 2025-06-17: customers may not "access the Services to build a competing product or service, including to train competing AI models or resell the Services except as expressly approved by Anthropic." Re-read the current text before relying on it; other providers' terms differ. <https://www.anthropic.com/legal/commercial-terms>

Seen as search-result summaries only (the primary page was blocked or not opened). The statements above are limited to what the summaries said. Open the paper before quoting it:

- Kapoor and Narayanan, "Leakage and the reproducibility crisis in machine-learning-based science", Patterns, 2023. <https://pmc.ncbi.nlm.nih.gov/articles/PMC10499856>
- METR, "Recent Frontier Models Are Reward Hacking", 2025-06-05. <https://metr.org/blog/2025-06-05-recent-reward-hacking>
- Beel, Kan, Baumgart, "Evaluating Sakana's AI Scientist", arXiv 2502.14297. <https://arxiv.org/abs/2502.14297>
- Xu et al., "Benchmark Data Contamination of Large Language Models: A Survey", arXiv 2406.04244. <https://arxiv.org/abs/2406.04244>
- Pineau et al., "Improving Reproducibility in Machine Learning Research", JMLR 22, paper 20-303, the NeurIPS reproducibility checklist, which asks for the number of models trained before the reported one. <https://www.jmlr.org/beta/papers/v22/20-303.html>
- Dror et al., "The Hitchhiker's Guide to Testing Statistical Significance in Natural Language Processing", ACL 2018. <https://aclanthology.org/P18-1128/>
- Bouthillier, Laurent, Vincent, "Unreproducible Research is Reproducible", ICML 2019: accounting for sources of variation, not only fixing seeds. <https://proceedings.mlr.press/v97/bouthillier19a.html>
- Longpre et al., Data Provenance Initiative, on growing `robots.txt` and terms-of-service restrictions on web data used for AI training. <https://openfuture.eu/note/consent-in-crisis-the-rapid-decline-of-the-ai-data-commons/>
</references>
