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

## Test Anti-Patterns

| Anti-Pattern | Problem | Better Approach |
|--------------|---------|-----------------|
| Testing implementation details | Breaks on refactor | Test behavior and a11y roles |
| Snapshot everything | Unreviewed noise | Assert specific values |
| Shared mutable state | Flaky order-dependent tests | Isolate setup per test |
| Mocking business logic | False confidence | Mock I/O only |
| Skipping tests to green CI | Hides failures | Fix, quarantine with ticket, or delete |

---

Repository-specific runners and examples: see linked profiles from [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md).
