# Security Checklist

Quick reference for application security. Use alongside the [`security-and-hardening`](../skills/security-and-hardening/SKILL.md) skill.

**Workspace-specific defaults** for the current project: [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md) — read applicable profile files (e.g. backend, frontend).

## Table of Contents

- [Pre-Commit Checks](#pre-commit-checks)
- [Authentication](#authentication)
- [Authorization](#authorization)
- [Input Validation](#input-validation)
- [Security Headers](#security-headers)
- [CORS Configuration](#cors-configuration)
- [Data Protection](#data-protection)
- [Dependency Security](#dependency-security)
- [Document and NoSQL Stores](#document-and-nosql-stores)
- [LLM / Agent Surfaces](#llm--agent-surfaces)
- [Error Handling](#error-handling)
- [OWASP Top 10 Quick Reference](#owasp-top-10-quick-reference)

## Pre-Commit Checks

- [ ] No secrets in code (`git diff --cached` review; use secret scanners in CI)
- [ ] `.gitignore` covers: `.env`, `.env.local`, `*.pem`, `*.key`
- [ ] `.env.example` uses placeholders only

## Authentication

- [ ] Passwords hashed with bcrypt (≥12 rounds), scrypt, or argon2
- [ ] Session cookies: `httpOnly`, `secure`, `sameSite` appropriate for your deployment
- [ ] Session expiration and rotation where applicable
- [ ] Rate limiting on login and sensitive endpoints
- [ ] Password reset tokens: time-limited, single-use
- [ ] MFA for sensitive operations when product requires it

## Authorization

- [ ] Every protected route and API checks authentication
- [ ] Resource access checks ownership/role (prevents IDOR)
- [ ] Admin paths require explicit role verification
- [ ] API keys scoped to minimum permissions
- [ ] JWTs validated (signature, `exp`, `iss`, `aud` if used)

## Input Validation

- [ ] **Frontend:** schema validation (e.g. form + client-parse libraries) at untrusted boundaries
- [ ] **Backend:** typed request models; reject unknown fields if policy requires
- [ ] Allowlists for enums and string patterns; length bounds on all strings
- [ ] File uploads: type allowlist, size limits, scan or sandbox if required
- [ ] SQL: parameterized queries (if any SQL in stack)
- [ ] HTML output escaped; avoid raw HTML sinks without sanitization
- [ ] Redirect URLs allowlisted (prevent open redirect)

## Security Headers

Deploy with sensible defaults (adjust for your app):

```
Content-Security-Policy: default-src 'self'; ...
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
```

## CORS Configuration

Restrict allowed origins in production; never use wildcard origins with credentials.

## Data Protection

- [ ] Sensitive fields omitted from API responses (`password_hash`, internal IDs if needed)
- [ ] No secrets or full PII in logs or analytics
- [ ] HTTPS for external traffic; TLS to the database when the provider requires it
- [ ] Backups encrypted at rest if regulated data exists

## Dependency Security

```bash
# JavaScript
npm audit
pnpm audit  # if using pnpm

# Python (example: uv project)
uv pip compile / uv sync per repo conventions
pip-audit  # or uv tool run pip-audit if configured
```

Keep lockfiles committed; review major upgrades.

## Document and NoSQL Stores

- [ ] No raw user JSON merged into query/update filters (NoSQL injection)
- [ ] Avoid server-side JS in queries, unbounded user regex, or user-built aggregation pipelines without allowlists
- [ ] Map validated DTOs to known-safe query shapes
- [ ] Least-privilege DB users (read-only vs read-write per service)

## LLM / Agent Surfaces

- [ ] Tool APIs allowlisted; no arbitrary shell or DB from model output
- [ ] Prompt injection defenses: system/developer boundaries, output validation for actions
- [ ] Rate limits and cost caps on LLM routes; audit logs for tool invocations

## Error Handling

Production responses must not leak stack traces, database internals, or secrets.

## OWASP Top 10 Quick Reference

| # | Vulnerability | Prevention |
|---|---------------|------------|
| 1 | Broken Access Control | AuthZ on every route; ownership checks |
| 2 | Cryptographic Failures | HTTPS, strong hashing, no secrets in repo |
| 3 | Injection | Parameterized queries; safe document queries; CSP |
| 4 | Insecure Design | Threat modeling; spec-driven requirements |
| 5 | Security Misconfiguration | Headers, minimal permissions, secure defaults |
| 6 | Vulnerable Components | Audit npm + Python (or stack) deps regularly |
| 7 | Auth Failures | Rate limits, session hygiene, MFA where needed |
| 8 | Data Integrity Failures | Signed artifacts; verify supply chain |
| 9 | Logging Failures | Security events logged; secrets never logged |
| 10 | SSRF | Allowlist URLs for outbound fetch from backend |

---

Repository-specific layering (client/server validation libraries, driver notes): add or follow entries in profiles linked from [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md).
