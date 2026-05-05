# Stack profile: Finance Analyzer

## Scope

Applies to research/analysis tasks involving public-company fundamentals, market context, and financial comparisons.

## When to load

- Financial statement analysis
- Ratio and trend analysis
- Macro + company evidence synthesis

## Detection

- Task mentions 10-K/10-Q/8-K, valuation, fundamentals, or market data
- Data sources include SEC/FRED/regulatory filings

## Framework and tooling

- Primary evidence sources: SEC EDGAR, regulator filings, FRED
- Secondary context: reputable financial journalism

## Repo layout conventions

- Read `README.md` and any data-source notes first
- Keep outputs traceable with explicit source links and as-of dates

## Commands

- `/research` for evidence gathering
- `/analyze` for recommendation synthesis
- `/investigate` for merged research + analysis + audit

## Boundaries

- No uncited numeric claims
- Use primary filings for core metrics where possible
- Label stale or uncertain values clearly

## References

- https://www.sec.gov/edgar
- https://www.sec.gov/data-research/sec-markets-data/financial-statement-data-sets
- https://fred.stlouisfed.org
- https://www.cfainstitute.org/ethics-standards/code-of-ethics-standards-of-conduct
