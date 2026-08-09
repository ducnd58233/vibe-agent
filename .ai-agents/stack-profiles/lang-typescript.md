# Stack profile: Language TypeScript / JavaScript

## Scope

<routing>

Applies to consumer repositories writing TypeScript or JavaScript at the language level: the type system, module resolution, compiler and lint configuration, and the JS semantics that survive compilation. Runtime concerns live in [`lang-nodejs.md`](lang-nodejs.md); framework concerns live in the matching frontend or backend profile.

## When to load

- Writing or reviewing TypeScript or JavaScript in any runtime
- Type modeling: generics, unions, narrowing, inference problems, declaration files
- `tsconfig.json`, module resolution, or build-output changes
- Publishing a package's type surface, or consuming an untyped dependency
- Migrating JavaScript to TypeScript, or tightening compiler strictness
</routing>

## Detection

<context>

- `tsconfig.json`, `jsconfig.json`, `*.ts`, `*.tsx`, `*.mts`, `*.cts`
- `package.json` with `types`/`typings`, `exports`, or a `typescript` dependency
- Lint and format config such as `eslint.config.*`, `.eslintrc.*`, `biome.json`, `.prettierrc*`
- `*.d.ts` files, `@types/*` dependencies

## Framework and tooling

Non-exhaustive examples. Any tool here may be renamed, deprecated, or replaced. Detect what the repo actually uses and verify current commands and flags against official docs ([`source-driven-development`](../skills/source-driven-development/SKILL.md)) before running anything.

- Compiler and type checking: `tsc`; type-aware lint rules where the repo enables them
- Lint and format: for example ESLint with typescript-eslint, Biome, Prettier
- Build and bundle: for example tsup, esbuild, Rollup, Vite, SWC, Babel
- Test: for example Vitest, Jest, node:test
- Validation at runtime boundaries: for example Zod, Valibot, ArkType, io-ts

## Repo layout conventions

- Read `tsconfig.json` first: `strict`, `target`, `module`, `moduleResolution`, and `paths` decide what is legal and how imports resolve
- In a monorepo, resolve which `tsconfig` actually applies to the file being edited; project references change the answer
- Keep the public surface of a package explicit through `exports` and emitted declarations, not through deep imports
- Match the file extension to the module system in use; `.mts`/`.cts` exist to disambiguate
</context>

## Commands

<procedure>

Use repo-documented commands first. Typical examples:

- Typecheck: `tsc --noEmit` (or the repo's typecheck script)
- Lint: the repo's lint script; type-aware rules need the project's `tsconfig`
- Test: the repo's test script
</procedure>

## Boundaries

<required>

- `any` erases checking and spreads silently. Prefer `unknown` at boundaries and narrow explicitly; when `any` is genuinely required, scope it as tightly as possible and say why
- Type assertions (`as`) and non-null assertions (`!`) claim knowledge the compiler does not have. Each one is a claim you must be able to justify; prefer narrowing, discriminated unions, or a schema check
- Types describe compile time only. Data crossing a runtime boundary — network, filesystem, environment, user input, `JSON.parse` — must be validated, not asserted
- Do not disable a compiler option or lint rule repo-wide to fix a local error
- Discriminated unions model mutually exclusive state better than optional fields; prefer them for state machines and result types
- Enabling `strict` changes semantics. Migrate incrementally by directory or file, not by turning it off
- JavaScript semantics still apply after compilation: coercion, `this` binding, prototype mutation, and floating-point precision are not type errors

## Security / performance appendix

- Validate and narrow untrusted input at the edge, then trust the narrowed type inward
- Avoid `eval`, dynamic `Function`, and untrusted dynamic `import()` targets
- Prefer structural narrowing over deep runtime clones in hot paths
- Type-level complexity has a compile-time cost; deeply recursive conditional types slow editors and CI
</required>

## References

<references>

- https://www.typescriptlang.org/docs/handbook/intro.html
- https://www.typescriptlang.org/tsconfig
- https://typescript-eslint.io/
- https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference
</references>
