# Testing Patterns Reference

Quick reference for tests across common web + API stacks. Use alongside the [`test-driven-development`](../skills/test-driven-development/SKILL.md) skill.

**Workspace-specific tooling** for the current project: [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md) — load every profile row that matches your slice.

## Table of Contents

- [Test Structure (Arrange-Act-Assert)](#test-structure-arrange-act-assert)
- [Test Naming Conventions](#test-naming-conventions)
- [Common Assertions](#common-assertions)
- [Mocking Patterns](#mocking-patterns)
- [React / Component Testing](#react--component-testing)
- [HTTP API Integration Testing](#http-api-integration-testing)
- [E2E Testing](#e2e-testing)
- [Core logic only (MUST)](#core-logic-only-must)
- [Measuring whether a test is worth keeping](#measuring-whether-a-test-is-worth-keeping)
- [Test Anti-Patterns](#test-anti-patterns)

## Test Structure (Arrange-Act-Assert)

```typescript
it('describes expected behavior', () => {
  const input = { title: 'Test Task', priority: 'high' }
  const result = createTask(input)
  expect(result.title).toBe('Test Task')
  expect(result.status).toBe('pending')
})
```

## Test Naming Conventions

```typescript
describe('TaskService.createTask', () => {
  it('creates a task with default pending status', () => {})
  it('throws ValidationError when title is empty', () => {})
})
```

## Common Assertions

Use your runner’s APIs (`expect` from Vitest/Jest or equivalent). Prefer deep equality for objects, strict checks for primitives, and `rejects` / `resolves` for async.

## Mocking Patterns

### Mock at Boundaries Only

```
Mock:                          Do not mock:
├── HTTP / fetch               ├── Pure utilities
├── DB / persistence driver    ├── Schema parse of fixed fixtures
├── External APIs              ├── Internal transformations
└── Clock (when needed)        └── Business rules you own
```

### Python (pytest)

Use fixtures for the ASGI/WSGI app + overridden dependencies; fake repositories implementing ports.

## React / Component Testing

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

it('submits the form with entered data', async () => {
  const onSubmit = vi.fn()
  render(<TaskForm onSubmit={onSubmit} />)
  await userEvent.type(screen.getByRole('textbox', { name: /title/i }), 'New Task')
  await userEvent.click(screen.getByRole('button', { name: /create/i }))
  expect(onSubmit).toHaveBeenCalledWith({ title: 'New Task' })
})
```

Prefer roles, labels, and `getByTestId` only where stable IDs exist per project rules.

## HTTP API Integration Testing

```python
import pytest
from httpx import ASGITransport, AsyncClient

@pytest.mark.anyio
async def test_create_task(async_client: AsyncClient):
    response = await async_client.post("/api/tasks", json={"title": "Test Task"})
    assert response.status_code == 201
    body = response.json()
    assert body["title"] == "Test Task"
```

Use dependency overrides or test containers to replace persistence with fakes or ephemeral databases.

## E2E Testing

```typescript
import { test, expect } from '@playwright/test'

test('user can create a task', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'New Task' }).click()
  await page.getByLabel('Title').fill('Buy groceries')
  await page.getByRole('button', { name: 'Create' }).click()
  await expect(page.getByText('Buy groceries')).toBeVisible()
})
```

Use web-first assertions; avoid hardcoded sleeps.

## Core logic only (MUST)

Tests must target **business behavior and core logic**. **Do not generate** meaningless discovery or infrastructure tests.

**MUST NOT appear in the suite:**

- File, folder, or path existence checks (`exists`, `access`, directory listing as the assertion)
- Environment variable or config file presence as the test body
- Testcontainer, Docker, or external service "is up" / connectivity-only cases
- Import/require smoke with no behavioral assertion
- Tests that only verify fixtures, `beforeAll`, or test harness startup
- Trivial code: getters, setters, constructors with no logic
- Wrappers and pass-through methods, asserted by checking the dependency was called
- Tautological assertions: a mock returning what the test told it to return, or a constant compared to itself

Use CI, global setup, or fixtures for infrastructure. Each test case should fail when **product logic** regresses.

**MUST be covered** when a change touches them: data ingestion and parsing of untrusted input, concurrency and replay, security boundaries and tenant isolation, money and state transitions, and failure paths. Banning low-value tests is not permission to write none.

**The bans are semantic, not syntactic.** A test may read a file, the clock, or the environment when that reading *is* the claim: "saving writes a manifest" and "this input creates no file" are behavior; "`config.json` exists" is discovery. The test is the question "does this fail when product behavior changes", not "which API did it call".

See [`test-driven-development`](../skills/test-driven-development/SKILL.md) for the full tables.

## Measuring whether a test is worth keeping

Coverage answers "was this line executed", which a useless test satisfies. Two better questions, in increasing cost:

**1. Would deleting the code fail the test?** This is *extreme mutation testing*: remove a function body and see whether the suite notices. A function that is covered but survives having its body deleted is **pseudo-tested** — the case exercised it and checked nothing that mattered. Niedermayr, Juergens and Wagner introduced the technique; across 19 open-source projects the median share of pseudo-tested methods was **10.1%** ([study](https://arxiv.org/pdf/2103.08480)). This is the mechanical form of the wrapper and pass-through bans above.

Tooling is per-stack and needs installing, so it is a deliberate opt-in rather than part of `make check`:

| Stack | Tool |
|-------|------|
| Java | [PIT](https://pitest.org/) with the [Descartes](https://inria.hal.science/hal-01870976/document) engine (pseudo-tested methods specifically) |
| Go | [gremlins](https://github.com/go-gremlins/gremlins) |
| JS/TS | [Stryker](https://stryker-mutator.io/) |
| Python | [mutmut](https://github.com/boxed/mutmut) |

**2. Has this test ever failed?** Coplien's framing: *"Tests that continually pass are producing no information — or at least very little information, and the value of the information they produce may not be worth the expense of maintaining and running the tests"* ([essay](https://wikileaks.org/ciav7p1/cms/files/Why-Most-Unit-Testing-is-Waste.pdf)). Treat it as a prompt to check the test can still fail, not as licence to delete a regression guard — see the [counter-argument](https://henrikwarne.com/2014/09/04/a-response-to-why-most-unit-testing-is-waste/).

The cheap signals — no assertion, an assertion only on the environment, an assertion only that a mock was called — are caught automatically by [`core-logic-test-guard.py`](../hooks/core-logic-test-guard.py) on every write to a test file.

## Test Anti-Patterns

| Anti-Pattern | Problem | Better Approach |
|--------------|---------|-----------------|
| Testing implementation details | Breaks on refactor | Test behavior and a11y roles |
| Snapshot everything | Unreviewed noise | Assert specific values |
| Shared mutable state | Flaky order-dependent tests | Isolate setup per test |
| Mocking business logic | False confidence | Mock I/O only |
| Infrastructure/discovery tests | No signal on logic regressions | Assert domain outcomes; keep infra in setup/CI |
| Testing a pass-through wrapper | Deleting the wrapper body may not fail the test | Test at the layer that owns the behavior |
| Tautological assertions | The test asserts what the test supplied | Assert an outcome the code computed |
| Skipping tests to green CI | Hides failures | Fix, quarantine with ticket, or delete |

---

Repository-specific runners and examples: see linked profiles from [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md).
