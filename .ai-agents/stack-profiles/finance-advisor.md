# Stack profile: Finance Advisor

## Scope

Applies to finance-oriented advisory-style synthesis where risk framing and compliance-safe communication are required.

## When to load

- Requests asking for financial recommendations
- Portfolio/allocation discussions
- Any user request that may be interpreted as investment advice

## Detection

- Task requests buy/sell/actionable advice
- Output may influence investment decisions

## Framework and tooling

- Same evidence stack as `finance-analyzer.md`
- Add advisor/regulatory framing guidance

## Repo layout conventions

- Read `README.md` and source policy first
- Include explicit disclaimer and risk framing in final output

## Commands

- `/research`
- `/analyze`
- `/investigate`

## Boundaries

- Always include educational-only disclaimer
- Refuse specific buy/sell/leverage directives
- Require caveats for horizon, risk tolerance, and uncertainty

## References

- https://www.sec.gov/edgar
- https://www.sec.gov/about/forms/formadv.pdf
- https://www.finra.org
- https://fred.stlouisfed.org
