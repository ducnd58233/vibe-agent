# Learning science behind the tutor

<context>

What the self-tutoring workflow is built on, and how much each piece of it can be trusted. This file exists so that a design rule in [`self-tutoring`](../skills/self-tutoring/SKILL.md) can be traced to its source, or honestly marked as a default someone chose.

**Nothing in the table below was read at its source in the build that wrote it.** The proxy blocked every paper. The findings were seen only in search summaries, so each is `UNVERIFIED`. They are included because the direction of each finding is well known in the field and is the reason for the design, but a number here must not be quoted as established until the paper is opened. To open them, add the hosts under Allowed domains in the environment settings, then run `vibe-agent docs check-citations` on a digest that cites them.
</context>

## Findings and what the tutor does with them

<rules>

| Finding (all `UNVERIFIED`) | Reported source | What the tutor does |
|---|---|---|
| Practice testing and distributed practice earned the highest utility ratings among ten study techniques. Summarisation, highlighting, and rereading earned low ratings. | Dunlosky and colleagues, 2013, in *Psychological Science in the Public Interest* | Every session starts and ends with the learner answering questions from memory. The tutor does not offer rereading or highlighting as study. |
| Spreading study over time beats massing it. The best gap between study sessions rose with how long the learner needs to remember, at about 10 to 20 percent of that interval. | Cepeda and colleagues, 2006, in *Psychological Bulletin* | Review dates are computed from the date the learner must know the material, not guessed. |
| Being tested helps later recall more than restudying the same material. | Roediger and Karpicke, 2006 | Retrieval comes before new material, and a wrong answer is corrected after the attempt. |
| In a field experiment with about a thousand high school students, an unguarded AI helper raised practice scores by about 48 percent and later exam-style scores fell about 17 percent once the helper was removed. A tutor prompted to withhold direct answers largely avoided the drop. | Bastani and colleagues, 2025, in *PNAS* | The tutor does not give an answer first. It uses a hint ladder, and the learner attempts every problem before seeing a solution. |

One more piece of design has no source here, and is labeled as a default the learner may change: a topic counts as learned only after the learner answers new questions correctly in two separate sessions at least three days apart, with at least 80 percent right each time.
</rules>

## The review dates, worked

<rules>

Given the date the learner must know the material, pick a gap of 10 to 20 percent of the time remaining, and recompute after each review. These lines are recomputed by `vibe-agent docs check-calcs`.

```calc
# 60 days to an exam: a gap of 10 to 20 percent is 6 to 12 days.
60 * 10% => 6
60 * 20% => 12
# A first review of new material on 2026-10-03, six days later.
todate(date("2026-10-03") + 6) => 2026-10-09
# After that review, 54 days remain, so the next gap is 5.4 to 10.8 days. Use 7.
54 * 10% => 5.4
54 * 20% => 10.8
todate(date("2026-10-09") + 7) => 2026-10-16
```

With no exam date, use a fixed ladder of 1, 3, 7, 14, and 30 days. That ladder is a common convention in flashcard practice and is not taken from a source opened here. A learner who misses a review restarts that item at the first rung.
</rules>

## References

<references>

None of these was opened. Open one before relying on a figure from it.

- https://pubmed.ncbi.nlm.nih.gov/26173288/ (Dunlosky et al., 2013)
- https://pubmed.ncbi.nlm.nih.gov/16719566/ (Cepeda et al., 2006)
- https://pubmed.ncbi.nlm.nih.gov/16507066/ (Roediger and Karpicke, 2006)
- https://www.pnas.org/doi/10.1073/pnas.2422633122 (Bastani et al., 2025)
- [`quantitative-accuracy`](../skills/quantitative-accuracy/SKILL.md) for how the dates and gaps above are computed
</references>
