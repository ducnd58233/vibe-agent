# Judgement: run 001

Hypothesis/assumption: Baseline accuracy would meet the project's floor of 0.80 on the v3 internal
eval set at these default decoding settings.
Observed: accuracy 0.84, latency_ms_p50 620 (metrics.json).
Verdict: confirmed
Why: 0.84 clears the 0.80 floor with margin, and nothing in this first run suggests the result is an
artifact of seed or sampling - there is no prior run-id to compare variance against yet.
Next: run 002 should vary temperature/top_k to see whether accuracy holds as decoding gets less
greedy, since this run only establishes that the conservative default clears the floor.
