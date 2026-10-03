# Resilience patterns

<context>

Design rules for software that has to keep working when a dependency is slow, wrong, or gone. Each rule says what to do and gives the reason, so it can be judged against your own case. The arithmetic is recomputed by `vibe-agent docs check-calcs`, so a figure here cannot drift from its formula.

**Source status.** The rules below are standard engineering practice. The primary sources that state them, the AWS Builders' Library on timeouts, retries, and backoff, and the Google SRE book on service level objectives, could not be opened in the build that wrote this file, so each is `UNVERIFIED` (see References). No figure in this file comes from them. The numbers are arithmetic on invented inputs. Where the rule is also visible in this repository's own code, the path is given and was read.

Related: [`concurrency-realtime-systems`](../skills/concurrency-realtime-systems/SKILL.md) for backpressure and slow consumers, [`observability-monitoring`](../skills/observability-monitoring/SKILL.md) for the signals, [`quantitative-accuracy`](../skills/quantitative-accuracy/SKILL.md) for the calculations.
</context>

## Rules

<required>

1. **Every call that leaves the process has a timeout.** A call with no timeout waits as long as the other side does, and a pile of waiting calls is how one slow dependency takes down the caller. Choose the timeout from the dependency's measured latency (a high percentile such as p99, from real traffic), not from a round number.
2. **Retry only what is safe to repeat.** A read is safe. A write is safe only if repeating it has the same effect, which means it is idempotent or carries an idempotency key the other side honours. Retrying a payment, a message send, or a counter increment without a key does it twice.
3. **Retry in one place.** If every layer of a call chain retries, the attempts multiply, and the bottom service sees the product of them at the moment it is already struggling. Pick the layer that knows the most about the failure and retry there.
4. **Back off, cap, and add jitter.** Wait longer after each failure, stop the wait growing at a cap, and randomise each wait so a crowd of clients that failed together does not return together.
5. **Bound the retries.** A fixed maximum number of attempts, and a total time that fits inside the caller's own deadline. A retry loop with no limit is an outage amplifier.
6. **Fail closed for a gate, fail open for bookkeeping.** A check that guards something irreversible refuses when it cannot answer. Journaling, metrics, and notes must never take down the work they describe.
7. **Degrade on purpose.** Decide in advance what the system does without each dependency: serve stale data, skip the optional part, queue the write, or refuse with a clear error. A decision made during the outage is made badly.
8. **Bound every queue and every input.** An unbounded queue, buffer, request body, or loop is a memory failure waiting for load. Reject or shed at a limit you chose.
9. **Make failure visible and testable.** Count timeouts, retries, and rejections. Test the failure paths by injecting them: a slow dependency, an error, a duplicate request, and a restart in the middle of an operation.
10. **State the objective, then spend the budget deliberately.** A service level objective (SLO) says how often the service may fail. The failures it allows are an error budget to spend on change, not a number to hide.
</required>

## Worked arithmetic

<rules>

Retries multiply across layers. Three attempts at each of five layers send up to 243 attempts to the bottom service for one user request.

```calc
3^5 => 243
```

A capped exponential backoff, starting at 100 ms, doubling, capped at 5000 ms. The wait before attempt `k` is the smaller of `100 * 2^k` and the cap.

```calc
min(100 * 2^0, 5000) => 100
min(100 * 2^1, 5000) => 200
min(100 * 2^2, 5000) => 400
min(100 * 2^3, 5000) => 800
min(100 * 2^4, 5000) => 1600
min(100 * 2^5, 5000) => 3200
min(100 * 2^6, 5000) => 5000
# Total time spent waiting across the first seven retries, in milliseconds.
sum(100, 200, 400, 800, 1600, 3200, 5000) => 11300
```

That total is 11.3 seconds of waiting before any call time is counted. If the caller's own deadline is 10 seconds, seven retries do not fit, and the retry count has to come down to what the deadline allows. Compute the budget before you set the count.

An SLO of 99.9 percent over a 30-day month allows this much downtime, in minutes, and a stricter SLO allows far less:

```calc
30 * 24 * 60 * (1 - 99.9%) => 43.2
30 * 24 * 60 * (1 - 99.99%) => 4.32
# A 20-minute incident against the 99.9 percent budget, as a percent of it.
20 / 43.2 * 100 => ~46.3
```

Dependencies in series multiply their availability. Three services, each at 99.9 percent, give less than 99.9 percent for a request that needs all three. State the combined figure before promising the single one.

```calc
99.9% * 99.9% * 99.9% * 100 => ~99.7
```
</rules>

## Where this repository already does it

<rules>

Each line points to code that was read in the build that wrote this file.

- Fail closed: the citation check at a research node refuses to leave the node when it cannot reach a URL (`runtime/internal/checkpoint/citations.go`). A broken consumer danger plan does not switch the built-in list off (`runtime/internal/harness/danger.go`).
- Fail open for bookkeeping: the opencode plugin never throws or rejects, and an absent binary makes every hook a quiet no-op (`.opencode/plugin/vibe-agent.js`).
- Bounded input: the calculator caps expression length, literal digits, nesting, exponent size, and the bit length of every intermediate value, and a fuzz run found one crash and no hang (`runtime/internal/calc/calc.go`).
- A budget on a loop: the agent loop has a turn, token, and wall-clock budget with a default turn ceiling when none is set, and a run has a maximum number of transitions (`runtime/internal/agent/app/loop.go`).
- Retry only with a changed attempt: the no-progress check refuses a retried artifact that repeats the failed one (`runtime/internal/noprogress/noprogress.go`).
</rules>

## Checklist for a new dependency

<verification>

- [ ] A timeout is set, and its value came from measured latency.
- [ ] Each retried operation is idempotent or carries an idempotency key.
- [ ] Retries happen at one layer, with a maximum count, a capped backoff, and jitter.
- [ ] The total retry time fits inside the caller's deadline, and the arithmetic is shown.
- [ ] The behaviour without the dependency is decided and written down.
- [ ] Queues, buffers, and inputs have limits, and the over-limit behaviour is chosen.
- [ ] Timeouts, retries, and rejections are counted.
- [ ] A test injects a slow call, an error, a duplicate request, and a restart mid-operation.
- [ ] The SLO and its budget in minutes are written down, with the combined figure for dependencies in series.
</verification>

## References

<references>

None of these was opened in the build that wrote this file, so each is `UNVERIFIED`. Open one before quoting a rule from it.

- https://aws.amazon.com/builders-library/timeouts-retries-and-backoff-with-jitter/
- https://aws.amazon.com/builders-library/making-retries-safe-with-idempotent-apis/
- https://sre.google/sre-book/service-level-objectives/
- https://sre.google/sre-book/addressing-cascading-failures/
</references>
