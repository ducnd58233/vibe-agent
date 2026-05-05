# References router

Lookup table for shared checklists and pattern docs under this folder. These files support skills (they are not Agent Skills `SKILL.md`).

**After you add, rename, or remove a reference file, update this table in the same change.**

| Topic | File | Primary skills |
|-------|------|----------------|
| WCAG 2.1 AA, keyboard and screen readers | [`accessibility-checklist.md`](accessibility-checklist.md) | `frontend-ui-engineering` |
| Core Web Vitals, frontend + API + DB | [`performance-checklist.md`](performance-checklist.md) | `performance-optimization` |
| Auth, validation, headers, CORS, OWASP | [`security-checklist.md`](security-checklist.md) | `security-and-hardening` |
| AAA, mocks, RTL, API, E2E | [`testing-patterns.md`](testing-patterns.md) | `test-driven-development` |
| Fan-out `/ship`, sequential lifecycle, anti-patterns | [`orchestration-patterns.md`](orchestration-patterns.md) | `using-agent-skills`, slash commands |

**Repository-specific tooling** for **this monorepo** is indexed in [`../stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md). Checklists here stay generic; load matching `*.md` profiles from that router when needed.
