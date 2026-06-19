# Stack profile: Frontend Next.js + TypeScript

## Scope

Applies to consumer repositories building web UI with Next.js and TypeScript, including App Router, validation, and client/server state boundaries.

## When to load

- UI or routing changes in Next.js apps
- Server/client component boundary decisions
- Form, state, and data-fetching architecture

## Detection

- `package.json` includes `next`
- `tsconfig.json` exists
- `app/` or `pages/` directory exists

## Framework and tooling

- Next.js
- TypeScript
- Optional conditional tools: Zod, Jotai, TanStack Query, SWR, Tailwind CSS

## Repo layout conventions

- Read `README.md`, `package.json`, `tsconfig.json`, `next.config.*` first
- Prefer route-grouped layout (`app/(group)/...`) when App Router is used
- Keep client components explicit with `'use client'`

## Commands

- `npm run lint`
- `npm run typecheck`
- `npm run test`
- `npm run build`

## Scaffolding & command surface (CLI-first)

Initialize and add libraries via official CLIs; do not hand-write the app skeleton, `next.config.*`, or component files from memory ([`source-driven-development`](../skills/source-driven-development/SKILL.md)):

- Init: `npx create-next-app@latest` (TypeScript + App Router per prompts)
- UI: `npx shadcn@latest init` → `npx shadcn@latest add <component>`
- Deps: `npm install <pkg>` / `pnpm add <pkg>` (e.g. `zod`, `@tanstack/react-query`)

**Node exception:** this is a JS project, so keep commands in `package.json` `scripts` (`dev`, `build`, `start`, `lint`, `typecheck`, `test`, and any `db:migrate*` for Prisma/Drizzle). Do **not** add a Makefile; `package.json` is the command surface. ORM migrations via the tool's CLI (e.g. `prisma migrate dev` / `drizzle-kit generate`), exposed as `package.json` scripts.

## Boundaries

- Keep server state in fetch/query layer, not UI-local state stores
- Keep runtime validation at boundaries (API, form parse, action parse)
- Avoid business logic inside route/page rendering components

## References

- https://nextjs.org/docs
- https://www.typescriptlang.org/docs/handbook/
- https://zod.dev
- https://jotai.org
- https://tanstack.com/query
