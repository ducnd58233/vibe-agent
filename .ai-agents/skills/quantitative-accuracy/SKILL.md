---
name: quantitative-accuracy
description: >-
  Rules for getting numbers right in research, analysis, docs, and answers:
  compute with vibe-agent calc instead of in your head, state units, currency
  and as-of dates, round once, compare like with like, and log every
  calculation so it can be rerun. Use whenever a figure is computed, converted,
  compared, or ranked, in finance, science, planning, or any other work.
disable-model-invocation: true
---

# Quantitative Accuracy

## Overview

<context>

A language model writes digits the way it writes words: by what usually comes next. That is why a long multiplication, a percentage of a percentage, or a date a week from now can be wrong in a fluent sentence. Two sources describe the fix, which is to have the model write the computation and a program run it: the PaL project ([README](https://github.com/reasoning-machines/pal)) and Anthropic's code execution tool ([docs](https://platform.claude.com/docs/en/agents-and-tools/tool-use/code-execution-tool)).

The toolkit ships that program as `vibe-agent calc`: exact arithmetic with explicit rounding, no binary-float drift, and bounded work. Syntax and limits: [`runtime/README.md`](../../../runtime/README.md) section "Calculate exactly". This skill is when to use it and how to state the result. It does not repeat the syntax.

The other half of accuracy is meaning, not arithmetic: the right units, the right period, the right comparison. A correct sum of numbers measured in different units is still wrong. A well-known case is a spacecraft lost to a mismatch between pound-force seconds and newton-seconds in a software file (Mars Climate Orbiter; `UNVERIFIED`: the NASA report was not opened in this build).
</context>

## The rules

<required>

1. **Do not compute in your head.** Any figure you produce that is not copied from a source is computed with `vibe-agent calc`. This includes sums, ratios, percentages, conversions, growth rates, averages, and dates. A single step you can see at a glance, such as `2 + 2`, is the only exception.
2. **No shell? Say so, and hand the work off.** A read-only agent has no shell. It must not do the arithmetic. It lists the calculations as a table with a column `Expression` and a column `Needed for`, and the main session runs them. If the runtime is not installed, write the figure with the label `UNVERIFIED (hand-computed)`.
3. **Every figure carries five things:** the value, its **unit**, its **currency** as an ISO 4217 code when it is money (USD, EUR, VND), its **as-of date or period**, and its **source** or the calculation that produced it.
4. **Log each calculation** in a fenced `calc` block so it can be rerun and checked:

   ````markdown
   ```calc
   (1250 - 1000) / 1000 * 100 => 25
   1000 / 7 => ~142.86
   ```
   ````

   `=> X` means exactly `X`. `=> ~X` means rounded to the decimals `X` shows. Run `vibe-agent docs check-calcs <file>`. A figure that is only quoted from a source is not a calculation and is not logged; it is cited.
5. **Round once, at the end.** Carry the full value between steps (`calc` does) and round only the figure you report. Never round a number and then add the rounded numbers.
6. **Say how you rounded.** The default is half-even. When a source says "rounded" and you must match it, write `[half_up]`. Report no more decimals than your least precise input supports.
7. **Never mix bases.** Write down, for every input: the unit, the period (a year, a fiscal year, a quarter, trailing twelve months), the scale (thousands, millions, billions), whether it is per share or in total, and whether it is nominal or adjusted for inflation. If two inputs differ in any of these, convert one with `calc` and show the conversion, or stop.
8. **Percent and percentage points are different things.** A rate that moves from 4% to 5% rose 1 percentage point and 25 percent. Write which one you mean.
9. **Cite the inputs you did not compute.** Open the source and check the figure beside the claim. A figure you could not trace is labeled `UNVERIFIED`.
10. **Lead with the answer.** One sentence with the figure, then one table of inputs and results. Do not restate the question or narrate the arithmetic.
</required>

## How to compute

<procedure>

1. **List the inputs** in a table: name, value, unit, currency, as-of, source.
2. **Write the formula** in words, then as one expression. Name every symbol.
3. **Run it:**

   ```sh
   vibe-agent calc "(150 / 100)^(1/3) - 1"
   ```

   Read the `exact` line. `no, the value does not end` means the printed figure is rounded and you must say so.
4. **Sanity-check, with a second route.** Pick one:
   - Undo it: if `growth = (end - start) / start`, then `start * (1 + growth)` must give `end`.
   - Bound it: the answer must lie between the smallest and largest plausible inputs, and have the right sign and size.
   - Compute it a second way and compare the two results.
5. **Log both** in a `calc` block, the calculation and its check.
6. **Write the result** with its unit, currency, as-of date, and source, as rule 3 says.

A worked example. A revenue line went from 100 to 150 over three years. The compound annual growth rate is the third root of the ratio, minus one:

```calc
# CAGR as a percent, rounded to two decimals.
((150 / 100)^(1/3) - 1) * 100 => ~14.47
# Check: growing 100 at that rate for three years must give back 150, to rounding.
100 * (1 + 14.47%)^3 => ~149.99
```

The check does not match 150 exactly because 14.47 is rounded, and the second line says so with `~`. That is the point of the two forms: an exact claim must be exact, and a rounded one must say it is rounded.
</procedure>

## Comparing things

<rules>

1. **Like with like.** Same unit, same period, same definition, same population. If a definition differs between sources, such as an adjusted figure against a reported one, show both and say which is which.
2. **Show the bases.** Give both values and the difference, as an absolute change and as a relative change. A relative change alone hides the size, and an absolute change alone hides the scale.
3. **Say what sets the ranking.** Name the criterion and the direction. If two items tie within the precision of the inputs, say they tie.
4. **Say how sure you are.** Give the sample size or the number of sources. One source is not a comparison, and a self-reported figure is not an independent one.
5. **Do not infer cause from a gap.** A difference is a difference. A cause needs its own evidence.
6. **Stale is a defect.** Compare figures from the same date. If they are not, say so and give both dates.
</rules>

## Antipatterns

<antipatterns>

| Wrong | Why it fails | Do this instead |
|-------|--------------|-----------------|
| Writing the product of two large numbers from memory | Digits come out fluent, not computed | `vibe-agent calc "<expr>"` |
| Rounding each line, then summing the lines | Rounding errors add up | Sum the full values, round the total |
| `Revenue grew 25%` with no base, period, or date | The reader cannot check or reuse it | State both values, the period, and the source |
| Comparing a trailing-twelve-month figure with a fiscal-year figure | Different periods | Convert to one basis, or compare only matched periods |
| "Up 1%" for a move from 4% to 5% | Percent and points are different | "Up 1 percentage point, or 25 percent" |
| Quoting a figure and logging it as a calculation | It was not computed, so a rerun means nothing | Cite it. Log only what you computed. |
| Writing `=> 25` for 25.4 | The line asserts exactness | `=> ~25` with the rounding stated, or the full value |
| A subagent doing arithmetic it has no tool for | Unchecked | List the calculations for the main session |
</antipatterns>

## Routing & discovery

<routing>

- **Use when:** a figure is computed, converted, compared, or ranked, in any domain.
- **Pairs with:** [`research-with-citations`](../research-with-citations/SKILL.md) for sources, [`evidence-based-analysis`](../evidence-based-analysis/SKILL.md) for decisions, and the [finance profiles](../../stack-profiles/finance-analyzer.md) for company and market work.
- **Do not use for:** reading a number straight from a source and quoting it with a citation. That needs the citation, not a calculation.
</routing>

## Permissions & authority

<required>

Uses `Bash` to run `vibe-agent calc` and `vibe-agent docs check-calcs`. Both are read-only and make no network call. Read-only agents have no `Bash`; rule 2 covers them.
</required>
