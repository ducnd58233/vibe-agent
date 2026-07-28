# Stack profile: Backend NestJS

## Scope

Applies to consumer repositories building backend services with NestJS: modules and dependency injection, controllers and providers, pipes, guards, interceptors, filters, and the transport layers Nest supports (HTTP, microservices, GraphQL, WebSockets). Compose with [`lang-typescript.md`](lang-typescript.md) for type-system work and [`lang-nodejs.md`](lang-nodejs.md) for runtime behavior.

## When to load

- Adding or restructuring Nest modules, controllers, providers, or resolvers
- Dependency-injection scope, provider lifetime, or circular-dependency problems
- Request pipeline work: validation pipes, guards, interceptors, exception filters, middleware
- Configuration, lifecycle hooks, or graceful shutdown
- Testing Nest units and end-to-end with the testing module

## Detection

- `package.json` depends on `@nestjs/core`, `@nestjs/common`
- `nest-cli.json`, `src/main.ts` calling `NestFactory`
- `*.module.ts`, `*.controller.ts`, `*.service.ts` naming; decorators such as `@Module`, `@Injectable`, `@Controller`
- Optional signals: `@nestjs/typeorm`, `@nestjs/mongoose`, `@nestjs/graphql`, `@nestjs/microservices`, `@nestjs/config`

## Framework and tooling

Non-exhaustive examples. Any package here may be renamed, deprecated, or replaced. Detect what the repo actually uses from its manifests and verify current APIs against official docs ([`source-driven-development`](../skills/source-driven-development/SKILL.md)) before using them.

- CLI and scaffolding: the Nest CLI for generating modules, controllers, and providers
- HTTP adapter: for example Express or Fastify — the adapter changes available request/response APIs
- Validation: for example class-validator and class-transformer driven by the global validation pipe, or a schema library the repo already uses
- Persistence: for example TypeORM, Prisma, Mongoose, Drizzle
- Config: for example `@nestjs/config` with schema validation at startup
- Test: for example Jest with `@nestjs/testing`, supertest for end-to-end

## Repo layout conventions

- Read `src/main.ts` and the root module first: global pipes, filters, prefixes, and adapter choice are set there
- Organize by feature module, each owning its controllers, providers, and DTOs; shared code goes in an explicit shared module
- DTOs define the transport contract; entities define persistence. Keep them separate even when the fields overlap
- Generate scaffolding with the Nest CLI rather than hand-writing wiring

## Commands

Use repo-documented commands first. Typical examples:

- Run: `nest start --watch` or the repo's dev script
- Build: `nest build`
- Test: `jest` unit runs and the repo's end-to-end configuration

## Boundaries

- Keep controllers thin: parse, delegate, return. Business logic belongs in providers, not in the request handler
- Validate at the edge with a pipe rather than inside services; a service should be able to trust its inputs
- Respect provider scope. Request-scoped providers propagate scope up the injection chain and cost performance — do not reach for them to pass request data around
- Resolve circular dependencies by extracting the shared concern into its own module, not by reaching for forward references as the default
- Do not import a feature module's internals; import the module and consume its exported providers
- Persistence entities must not leak to the transport layer as response types; map explicitly
- Implement graceful shutdown through Nest's lifecycle hooks so in-flight requests drain before the process exits

## Security / performance appendix

- Enable the global validation pipe with whitelisting so unknown properties are stripped rather than silently accepted
- Authorization belongs in guards, applied deliberately per route or globally with explicit exceptions — not scattered through services
- Never return an entity containing secrets or password hashes; use an explicit response DTO or serialization interceptor
- Set timeouts and bound payload size at the adapter level
- Watch for N+1 queries introduced by lazy relations in the ORM; verify with query logging

## References

- https://docs.nestjs.com/
- https://docs.nestjs.com/fundamentals/injection-scopes
- https://docs.nestjs.com/techniques/validation
- https://docs.nestjs.com/fundamentals/testing
