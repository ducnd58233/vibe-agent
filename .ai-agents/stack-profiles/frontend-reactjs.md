# Stack profile: Frontend React.js

## Scope

Applies to consumer repositories building browser UI with React outside a more specific framework profile such as Next.js, including Vite, CRA, Remix-like client code, component libraries, forms, state, and data-fetching integration.

Compose with [`lang-typescript.md`](lang-typescript.md) for type-system concerns.

## When to load

- React component, hook, page, or client-side routing work
- UI state, server-state cache, form, accessibility, or rendering performance decisions
- Vite / SPA / component-library changes where no more specific framework profile applies

## Detection

- `package.json` includes `react` and `react-dom`
- `vite`, `@vitejs/plugin-react`, `react-router`, `tanstack`, `swr`, `zustand`, `redux`, `storybook`, or `vitest`
- Source paths such as `src/App.*`, `src/components/`, `src/features/`, `src/routes/`, `src/hooks/`

## Framework and tooling

- React with TypeScript when `tsconfig.json` exists
- Vite or repo-pinned bundler/test runner
- Optional: React Router, TanStack Query, SWR, Zod, React Hook Form, Zustand/Redux, Storybook, Testing Library
- Accessibility tooling: eslint-plugin-jsx-a11y, axe, Playwright/Cypress where configured

## Repo layout conventions

- Read `package.json`, lockfile, `tsconfig.json`, bundler config, and test config first
- Prefer feature-first slices for product code; keep shared primitives/design-system components deliberate
- Keep presentational components separate from data-fetching/container logic when complexity grows
- Keep server state in query/cache libraries or route loaders, not ad hoc global stores
- Use semantic HTML and accessible labels before adding ARIA

## Commands

- `npm run lint`
- `npm run typecheck`
- `npm run test`
- `npm run build`

## Boundaries

- Do not introduce framework-specific assumptions such as Next.js server components unless detected
- Do not deep-import sibling feature internals; route through shared contracts or colocated hooks
- Do not store remote/server data as duplicated local UI state without a cache invalidation plan
- Do not add broad state libraries when local state, URL state, or a query cache is enough

## Security / performance appendix

- Validate and encode untrusted rendered content; avoid unsafe HTML unless sanitized
- Measure rendering regressions with React Profiler or browser tooling before large memoization changes
- Stabilize list keys, virtualize large lists, split heavy bundles, and keep suspense/loading/error states explicit

## References

- https://react.dev/learn
- https://react.dev/learn/managing-state
- https://react.dev/reference/react
- https://vite.dev/guide/
- https://testing-library.com/docs/react-testing-library/intro/
