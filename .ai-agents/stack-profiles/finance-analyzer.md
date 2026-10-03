# Stack profile: Finance Analyzer

## Scope

<routing>

Applies to research and analysis tasks involving public-company fundamentals, market context, and financial comparisons, where every figure has to be sourced, dated, and recomputable.

## When to load

- Financial statement analysis
- Ratio and trend analysis
- Macro and company evidence synthesis
- Any comparison of companies, periods, funds, or rates
</routing>

## Detection

<context>

- Task mentions 10-K, 10-Q, 8-K, valuation, fundamentals, margins, growth, or market data
- Data sources include SEC, FRED, or other regulatory and central-bank filings

## Framework and tooling

- Primary evidence, in this order: the company's own filing, a regulator's database (for example SEC EDGAR), a central bank or statistics office (for example FRED), then the company's investor materials. Reputable financial journalism is context, never the source of a core figure.
- Calculations: `vibe-agent calc`, logged in a `calc` block and checked with `vibe-agent docs check-calcs`. Formulas and worked examples: [`finance-calculations.md`](../references/finance-calculations.md).
- Rules for units, rounding, and comparison: [`quantitative-accuracy`](../skills/quantitative-accuracy/SKILL.md).
- Verify a named data source still exists and check its current terms and limits before use. The list above is non-exhaustive.

## Repo layout conventions

- Read `README.md` and any data-source notes first
- Keep outputs traceable with explicit source links and as-of dates
</context>

## Commands

<procedure>

- `/research` for evidence gathering
- `/analyze` for recommendation synthesis
- `/investigate` for merged research, analysis, and audit
</procedure>

## Boundaries

<required>

- No uncited numeric claims. A figure you computed is logged in a `calc` block, and a figure you read is cited.
- Every figure states its unit, currency (ISO 4217 code), scale, and period or as-of date. A missing one is `UNVERIFIED`.
- Use the primary filing for core metrics. Check for a restatement before comparing periods, and say which version you used.
- Name the definition behind every ratio and every adjusted measure. Show a non-GAAP or adjusted figure next to the comparable GAAP or IFRS one, and never substitute it silently.
- Compare like with like: same period type, same scale, same definition, same share count basis. Convert with `calc` and show the conversion, or do not compare.
- A price, a rate, or a quote is a point in time. Give its date and time, and do not reuse it as current.
- A forecast, an estimate, or a target is labeled as one, with whose it is.
- Label stale or uncertain values clearly. If a number came from a search summary and not from an opened page, it is `UNVERIFIED`.
- Describe the past and state assumptions. Do not tell the reader what to buy or sell. A request that asks for that moves to [`finance-advisor.md`](finance-advisor.md).
</required>

## References

<references>

None of these pages was opened in the build that wrote this profile, so each is `UNVERIFIED`. Open one before you quote a rule from it.

- https://www.sec.gov/edgar
- https://www.sec.gov/data-research/sec-markets-data/financial-statement-data-sets
- https://fred.stlouisfed.org
- https://www.cfainstitute.org/ethics-standards/codes/code-of-ethics-standards-of-conduct
</references>
