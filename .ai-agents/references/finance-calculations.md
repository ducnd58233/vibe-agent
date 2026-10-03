# Finance calculations

<context>

Formulas and worked examples for the figures that finance research and analysis use most. Every example is recomputed by `vibe-agent docs check-calcs`, and `vibe-agent doctor` recomputes them all, so a formula here cannot drift from its answer.

**All example inputs are invented for illustration. They are not market data and not a quote.** Real inputs come from a source you opened, with their unit, currency, and as-of date, as [`quantitative-accuracy`](../skills/quantitative-accuracy/SKILL.md) requires.

How to read a line: `expression => value` means exactly that value, and `expression => ~value` means rounded to the decimals shown. Syntax: [`runtime/README.md`](../../runtime/README.md) section "Calculate exactly".
</context>

## Before you calculate

<required>

1. **Write the definition you use.** Companies and data providers define the same label differently: enterprise value, free cash flow, EBITDA, net debt, "adjusted" earnings. State the definition and the source of each input next to the result. Two figures with one label and two definitions are not comparable.
2. **Fix the period.** Say whether a figure is a fiscal year, a calendar year, a quarter, or trailing twelve months, and the period end date. Do not compare across different period types.
3. **Fix the scale and currency.** Thousands, millions, or billions; ISO 4217 code. Convert before you combine, and show the conversion.
4. **Prefer the primary filing.** Take reported figures from the company's own filing, not from a summary of it. Check whether the figure was later restated, and use the restated one when you compare periods, saying so.
5. **Label reported and adjusted figures.** A non-GAAP or "adjusted" measure is shown beside the most directly comparable GAAP or IFRS measure, with the reconciliation if the filing gives one, and is never silently substituted for it. Regulatory basis: see References, `UNVERIFIED` until the pages are opened.
6. **Use the share count the measure needs.** Basic and diluted share counts give different per-share figures. Say which, and the date of the count.
7. **Do not turn a calculation into advice.** A computed figure describes the past or states an assumption. See [`finance-advisor`](../stack-profiles/finance-advisor.md) for requests that ask what to buy or sell.
</required>

## Growth and change

<rules>

Percent change is the new value minus the old, over the old.

```calc
# Revenue 120 to 150, as a percent.
(150 - 120) / 120 * 100 => 25
```

Percentage points are the plain difference between two rates. They are not the same as percent change. A margin that goes from 8.0% to 10.0% rose 2 percentage points, and 25 percent in relative terms.

```calc
10 - 8 => 2
(10 - 8) / 8 * 100 => 25
```

Compound annual growth rate over `n` years is the `n`-th root of the end-to-start ratio, minus one. It needs the number of years between the two dates, not the number of data points.

```calc
# 100 growing to 150 over 3 years, as a percent.
((150 / 100)^(1/3) - 1) * 100 => ~14.47
# Check: compounding 100 at that rate for 3 years returns about 150.
100 * (1 + 14.47%)^3 => ~149.99
```

Compounding an amount forward is `amount * (1 + rate)^years`.

```calc
1000 * (1 + 5%)^10 => ~1628.89
```
</rules>

## Profitability and per-share figures

<rules>

```calc
# Net margin: net income 18 on revenue 240, as a percent.
18 / 240 * 100 => 7.5
# Earnings per share: net income 18,000,000 over 12,000,000 shares.
18000000 / 12000000 => 1.5
```

Price-to-earnings is price over earnings per share. Its reciprocal, earnings yield, must agree with it, which makes a cheap check.

```calc
# P/E at a price of 30 and EPS of 1.5.
30 / 1.5 => 20
# Earnings yield as a percent, and its check against 100 / (P/E).
1.5 / 30 * 100 => 5
100 / 20 => 5
```

State whether EPS is trailing twelve months, the last fiscal year, or a forecast. A P/E on a forecast is a forecast.
</rules>

## Value and leverage

<rules>

Market capitalisation is price times shares outstanding. Enterprise value adds debt and subtracts cash, **in the definition you state**: some analysts also add preferred stock and minority interests. Match the numerator and the denominator: EV pairs with a pre-interest measure such as EBITDA, and market capitalisation pairs with a post-interest measure such as net income.

```calc
# Market cap at a price of 30 and 12,000,000 shares.
30 * 12000000 => 360000000
# Enterprise value: cap 360,000,000 plus debt 90,000,000 minus cash 30,000,000.
360000000 + 90000000 - 30000000 => 420000000
# EV to EBITDA with EBITDA of 60,000,000.
420000000 / 60000000 => 7
```
</rules>

## Time value of money

<rules>

Net present value discounts each cash flow by `(1 + rate)^t`, where `t` is the number of periods from the start. The rate and the period must use the same unit: an annual rate for yearly flows, a monthly rate for monthly flows.

```calc
# Rate 10%, flows of -100 now, then 60 after one year and 60 after two.
-100 + 60 / (1 + 10%) + 60 / (1 + 10%)^2 => ~4.13
```

A level loan payment is `principal * r / (1 - (1 + r)^-n)`, with `r` the rate per period and `n` the number of periods. For 6% a year paid monthly over 30 years, `r` is 0.5% and `n` is 360. This is the payment per 10,000 borrowed, before any fees or taxes.

```calc
10000 * 0.005 / (1 - (1 + 0.005)^-360) => ~59.96
```

A real return removes inflation by division, not subtraction. A 7% nominal return with 3% inflation is 3.88% real, not 4%.

```calc
((1 + 7%) / (1 + 3%) - 1) * 100 => ~3.88
```
</rules>

## Currency and weighted averages

<rules>

Say which way a rate points. An exchange rate of 1.10 dollars per euro converts euros to dollars by multiplying, and dollars to euros by dividing. State whether the rate is a mid rate, a buy or sell rate, or a daily reference rate, and its date.

```calc
# 1000 EUR at 1.10 USD per EUR.
1000 * 1.10 => 1100
# The inverse rate, in EUR per USD.
1 / 1.10 => ~0.9091
```

A portfolio's return is the weighted average of its parts, with weights that sum to one. Hold the weights as of the same date as the returns.

```calc
# 60% at 8% and 40% at 2%.
0.6 * 8 + 0.4 * 2 => 5.6
```
</rules>

## References

<references>

None of these pages was opened in the build that wrote this file, so each is `UNVERIFIED`. Open and read the page before quoting a rule from it.

- SEC EDGAR APIs, for filing data and fair-access limits: https://www.sec.gov/search-filings/edgar-application-programming-interfaces
- SEC staff interpretations on non-GAAP financial measures: https://www.sec.gov/rules-regulations/staff-guidance/compliance-disclosure-interpretations/non-gaap-financial-measures
- Federal Reserve Economic Data API: https://fred.stlouisfed.org/docs/api/fred/
- Python `decimal`, on why decimal arithmetic suits money: https://docs.python.org/3/library/decimal.html
- [`finance-analyzer`](../stack-profiles/finance-analyzer.md) and [`finance-advisor`](../stack-profiles/finance-advisor.md) for the research and advice boundaries
</references>
