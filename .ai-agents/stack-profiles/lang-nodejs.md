# Stack profile: Runtime Node.js

## Scope

<routing>

Applies to consumer repositories running JavaScript or TypeScript on Node.js at the runtime level: the event loop, streams and backpressure, module systems, process and worker model, package management, and native addons. Language rules live in [`lang-typescript.md`](lang-typescript.md); HTTP framework rules live in the matching backend profile.

## When to load

- Server, CLI, script, or tooling work executing on Node.js
- Event-loop blocking, memory growth, or throughput problems
- Stream, buffer, or backpressure handling
- ESM and CommonJS interop, module resolution, or `exports` map changes
- Package management, workspaces, lockfiles, or dependency install behavior
- Child processes, worker threads, clustering, or graceful shutdown
</routing>

## Detection

<context>

- `package.json` with `engines.node`, `"type"`, `bin`, or `exports`
- Lockfiles such as `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lockb`
- `.nvmrc`, `.node-version`, `node_modules/`
- Imports of `node:` builtins, `worker_threads`, `child_process`, `cluster`

## Framework and tooling

Non-exhaustive examples. Any tool here may be renamed, deprecated, or replaced. Detect what the repo actually uses from its lockfile and scripts, and verify current commands and flags against official docs ([`source-driven-development`](../skills/source-driven-development/SKILL.md)) before running anything.

- Package managers: for example npm, pnpm, Yarn, Bun - the lockfile present decides which one the repo uses
- Version management: for example nvm, fnm, Volta, or the `engines` field
- Process management in production: for example systemd, PM2, container orchestration
- Diagnostics: built-in `--inspect`, `--cpu-prof`, `--heap-prof`, `node:diagnostics_channel`; for example clinic.js for higher-level analysis
- Test runners: for example `node:test`, Vitest, Jest

## Repo layout conventions

- Read `package.json` first: `type`, `engines`, `exports`, `bin`, and `scripts` define how the code loads and runs
- Prefer the repo's existing script names over invoking tools directly; scripts encode required flags
- Never change the package manager or regenerate a lockfile as a side effect of another task
- Keep runtime configuration in environment variables read at startup, not scattered through modules
</context>

## Commands

<procedure>

Use repo-documented commands first. Typical examples:

- Install with the package manager matching the committed lockfile
- Run and test through the repo's `scripts` entries
- Profile with the built-in flags before adding a profiling dependency
</procedure>

## Boundaries

<required>

- The event loop is single-threaded per process. Synchronous CPU-bound work, large synchronous JSON handling, and sync filesystem calls block every pending request - move them to worker threads or a separate process
- Honor stream backpressure: piping without respecting it turns a slow consumer into unbounded memory growth
- Never mix `await` with unhandled promise rejection paths; an unhandled rejection terminates the process by default in current Node versions
- ESM and CommonJS are not interchangeable. Named exports from CJS are not always statically analyzable, `require` of ESM is restricted, and `__dirname` has no direct ESM equivalent - resolve interop deliberately
- Implement graceful shutdown: stop accepting work, drain in-flight requests, close resources, then exit. Abrupt exit loses in-flight state
- Do not add a dependency for something the standard library already provides
- Treat `process.env` as untrusted strings: absent, empty, and malformed are all possible; validate at startup and fail fast

## Security / performance appendix

- Pin and audit dependencies; install scripts execute arbitrary code at install time
- Never interpolate untrusted input into shell commands; prefer argument arrays over a shell string
- Validate and bound request body size, upload size, and concurrency before doing work
- Set explicit timeouts on every outbound call; a hung dependency otherwise exhausts the pool
- Measure with a profiler before optimizing; event-loop delay and heap growth are the two signals worth watching first
</required>

## References

<references>

- https://nodejs.org/docs/latest/api/
- https://nodejs.org/en/learn/asynchronous-work/event-loop-timers-and-nexttick
- https://nodejs.org/en/learn/modules/publishing-node-api-modules
- https://docs.npmjs.com/cli/configuring-npm/package-json
</references>
