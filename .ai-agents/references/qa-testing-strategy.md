# QA testing strategy

Use this reference for manual QA, automated test planning, exploratory testing, regression strategy, and release confidence.

## Test strategy layers

- **Static checks:** lint, typecheck, schemas, contracts, accessibility/static security checks.
- **Unit tests:** pure logic and small components.
- **Integration tests:** API, database, message, file, and framework boundaries.
- **E2E tests:** critical user journeys with real browser/device/runtime.
- **Exploratory/manual testing:** chartered sessions for usability, edge cases, visual issues, and unknown unknowns.
- **Non-functional testing:** accessibility, security, performance, reliability, compatibility, localization, mobile/browser matrix.

## Manual QA workflow

1. Define a test charter: mission, scope, risks, environment, data, and timebox.
2. Create high-value scenarios: happy path, boundary values, error/retry, permissions, offline/slow network, interrupt/resume.
3. Execute with evidence: screenshots, logs, URLs, versions, test data, reproduction steps.
4. Classify findings by severity and user impact.
5. Convert recurring manual checks into automation when stable and valuable.

## Automation workflow

1. Pick the lowest test level that proves behavior.
2. Prefer user-observable assertions over implementation details.
3. **Core logic only:** do not add automated tests for file/folder/env discovery, testcontainer health, or setup-only checks — see [`test-driven-development`](../skills/test-driven-development/SKILL.md).
4. Use stable selectors/roles/labels and web-first assertions for browser automation.
5. Avoid hard sleeps; wait on observable state.
6. Keep tests isolated, deterministic, and parallel-safe.
7. Track flaky tests separately; do not normalize reruns as success.

## Release QA checklist

- [ ] Acceptance criteria mapped to tests or manual charters.
- [ ] Critical user journeys automated or manually verified.
- [ ] Accessibility keyboard/screen-reader basics checked for UI changes.
- [ ] Security test scope identified for auth/data/input changes.
- [ ] Performance smoke/budget checked for hot paths.
- [ ] Browser/device/platform matrix chosen based on actual users.
- [ ] Bugs include reproducible steps and evidence.

## References

- https://playwright.dev/docs/best-practices
- https://playwright.dev/docs/test-assertions
- https://testing-library.com/docs/guiding-principles
- https://owasp.org/www-project-web-security-testing-guide/
- https://astqb.org/4-4-experience-based-test-techniques/
