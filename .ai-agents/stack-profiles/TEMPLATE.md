# Stack profile authoring template

Use this contract when **adding a new stack profile**. Implement each profile as **`stack-profiles/<short-name>.md`** (standalone markdown—not a `SKILL.md`). Skills stay stack-agnostic; profiles pin **names, paths, tooling, and repo-specific conventions** discoverable via [`ROUTER.md`](ROUTER.md).

After clone, run [`scripts/link-ai-agents.ps1`](../../scripts/link-ai-agents.ps1) (or `.sh`) where relevant; profiles are **not** mirrored into `.cursor/skills`—consumers reach them via links from skills and ROUTER.

---

## What

- **File name:** `stack-profiles/<slug>.md` (e.g. `backend-fastapi.md`, `frontend-nextjs.md`, `devops-docker.md`)
- **Purpose:** Codify frameworks, dependency pins, repo layout assumptions, run commands, and security/perf quirks for **one logical layer or concern**.
- **Inputs:** Maintainer knowledge of how this repo ships that slice.
- **Outputs:** Agents and humans pick the right profiles from [`ROUTER.md`](ROUTER.md) instead of scattering stack tables across skills.

---

## Why

- **Problem:** Generic skills drift if every file repeats “we use framework X”.
- **Success criteria:** A new contributor can locate **all** applicable profiles in one ROUTER row and compose them (frontend + backend + devops).
- **Non-goals:** Full product/domain charter—that stays in root [`AGENTS.md`](../../AGENTS.md).

---

## Required sections in each `*.md` profile

Structure the markdown file with **these headings** (in order):

1. **`# Stack profile: <title>`**
2. **`## Scope`** — One paragraph: frontend / backend HTTP / backend gRPC / full-stack slice / CI / infra, etc.
3. **`## When to load`** — Bullet triggers (e.g. “touching `apps/web/`”, “changing API routes”).
4. **`## Detection`** — Signals in repo (manifests, directories, binaries) suggesting this profile applies.
5. **`## Framework and tooling`** — Named libraries, validators, routers, runners (as appropriate).
6. **`## Repo layout conventions`** — Important paths (`apps/`, `backend/`, `services/`), where tests live.
7. **`## Commands`** — Lint, test, typecheck documented for **this slice** only.
8. **`## Boundaries`** — Allowed vs forbidden coupling (what must not bleed into adjacent layers).

Optional:

- **`## Security / performance appendix`** — Checklist deltas for this slice.
- **`## References`** — Official doc URLs for pinned versions.

---

## When

**Load when:** Tasks cross into implementation details tied to pinned frameworks named in ROUTER profiles.

**Do not use as:** Sole substitute for skills—skills define **workflow**; profiles define **this repo’s pinned stack**.

---

## Routing & discovery

Draft the **YAML-free** markdown so [`ROUTER.md`](ROUTER.md) can summarize it in table form.

Suggested ROUTER columns (see template row when you add yours):

| Profile file | Layer / concern | When to load | Detection / notes |

---

## After creating (MUST)

1. Update **[`ROUTER.md`](ROUTER.md)** in **this folder** in the **same change**: add **one row** per new profile (Profile file path, Layer / concern, When to load, Detection / notes); remove rows when deleting a profile.
2. Optionally add a sentence in root [`README.md`](README.md) only if onboarding needs a spotlight—keep the **authoritative index** in `ROUTER.md`.
3. Mention new profiles from root [`AGENTS.md`](../../AGENTS.md) if they change **human** onboarding expectations.
