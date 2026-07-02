---
name: qa-testing-strategy
description: >-
  Designs and executes manual and automated QA strategy for product quality: test charters, exploratory testing, regression suites, E2E automation, acceptance coverage, bug reproduction, cross-browser/mobile checks, accessibility/security/performance smoke testing, and release signoff. Use when planning or auditing manual QA, automation QA, test coverage, test matrices, or tester workflows.
disable-model-invocation: true
---

# QA Testing Strategy

## What

Plan, execute, and review manual and automated testing for release confidence across functional, exploratory, regression, compatibility, accessibility, security, and performance concerns.

## Why

TDD proves code behavior at implementation time; QA strategy proves user-facing quality, risk coverage, release readiness, and manual-to-automation conversion. Expert testers combine scripted checks, exploratory charters, automation, evidence capture, and risk-based prioritization.

**Automated tests in code** follow [`test-driven-development`](../test-driven-development/SKILL.md): core/business logic only — no file/env/container discovery tests. Environment and infra checks belong in manual charters, CI, or fixture setup, not unit/integration test bodies.

## How

1. **Load context**
   - Read acceptance criteria, specs, risk areas, target users, supported browsers/devices, existing tests, and applicable stack profiles.
   - Use [`references/qa-testing-strategy.md`](../../references/qa-testing-strategy.md) and [`references/testing-patterns.md`](../../references/testing-patterns.md).
2. **Define risk and coverage**
   - Map features to happy paths, edge cases, permissions, errors, data boundaries, localization, accessibility, performance, security, and platform matrix.
3. **Split manual vs automation**
   - Automate stable, repeatable, high-value regression paths.
   - Use manual/exploratory testing for new UX, ambiguous behavior, visual quality, complex workflows, and unknown risks.
4. **Create test artifacts**
   - Manual: charter, scenarios, test data, environment, expected observations, evidence checklist.
   - Automation: test level, selectors/fixtures, assertions, data setup/teardown, flake controls, CI command.
5. **Execute with evidence**
   - Capture steps, environment, screenshots/logs/network traces, versions, data IDs, and expected vs actual behavior.
6. **Close the loop**
   - Prioritize defects by user impact.
   - Convert recurring manual cases into automation.
   - Update regression suites and release signoff criteria.

## When

Use for manual QA, automation QA, tester workflows, release testing, coverage audits, exploratory testing, and cross-platform test matrices. Use [`test-driven-development`](../test-driven-development/SKILL.md) when writing tests first for implementation behavior.

## Routing & discovery

- Pair with [`browser-testing-with-devtools`](../browser-testing-with-devtools/SKILL.md) for browser runtime evidence.
- Pair with [`frontend-ui-engineering`](../frontend-ui-engineering/SKILL.md) for accessibility and UI quality.
- Pair with [`shipping-and-launch`](../shipping-and-launch/SKILL.md) for release signoff.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for auth/data/input test scope.

## Permissions & authority

- Tools: Read, Grep, Glob, Edit; Shell for repo-documented test commands; browser/devtools tools when available.
- Paths: tests, specs, QA docs, fixtures, reports; no secrets or private user data.
- Ask before long E2E runs, external-service tests, production/staging mutations, or device-farm/cloud test jobs.

## Verification

- [ ] Acceptance criteria map to manual or automated coverage.
- [ ] Critical journeys have clear evidence and signoff status.
- [ ] Automated tests use stable selectors and observable assertions, not implementation details.
- [ ] Manual findings include reproducible steps and environment details.
- [ ] Flaky or skipped tests are tracked, not ignored.

## References

- https://playwright.dev/docs/best-practices
- https://testing-library.com/docs/guiding-principles
- https://owasp.org/www-project-web-security-testing-guide/
- https://astqb.org/4-4-experience-based-test-techniques/
