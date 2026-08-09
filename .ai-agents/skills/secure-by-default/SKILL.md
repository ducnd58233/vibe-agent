---
name: secure-by-default
description: >-
  Applies write-time secrecy constraints so generated code never puts credentials, tokens,
  or user data where an end user, an outside developer, or an attacker can read them: client
  bundles, device storage, consoles and device logs, rendered UI, API responses, server logs,
  telemetry, and build artifacts. Use during any implementation, refactor, test, or review
  that touches auth, user data, logging, error handling, configuration, or a client surface.
---

# Secure by Default

## Overview

<context>

This skill is a **write-time** constraint, not a review-time checklist. It applies while code is
being produced, because a leak that reaches a commit has already been published to anyone with
repository access, and a leak that reaches a client bundle has been published to the world.

The governing question is one line:

> Can anyone who is not this specific user read this value?

"Anyone" includes the ordinary end user, an outside developer reading a bundle, a support engineer
reading a log, and an attacker. The four are the same threat for this purpose, because none of them
should see the value and none of them is stopped by intent.

The deep material is in [`security-and-hardening`](../security-and-hardening/SKILL.md) and
[`references/security-checklist.md`](../../references/security-checklist.md). Channel-by-channel
detail is in [`references/sensitive-data-exposure.md`](../../references/sensitive-data-exposure.md).
This skill is the part that must be loaded before those are consulted, so that the agent knows to
consult them.

These constraints are **harness-agnostic**: apply them identically across Claude, Codex, Cursor,
opencode, and any other tool, and to both primary agents and subagents.

## What counts as sensitive

Treat as sensitive unless the value is provably public:

| Class | Examples |
|---|---|
| Authentication material | Passwords in any form, session tokens, refresh tokens, JWTs, cookies carrying auth |
| Machine credentials | API keys, client secrets, signing keys, private keys, database URLs with a password |
| Personal data | Names tied to accounts, email, phone, address, government IDs, location, health, payment |
| Internal structure | Stack traces, file paths, SQL text, internal hostnames, queue names, primary keys of other tenants |
| Derived signals | Anything from which the above can be reconstructed, including full request bodies and unfiltered object dumps |

When a value's class is genuinely unclear, treat it as sensitive and say so. The cost of a wrong
"sensitive" call is one extra redaction. The cost of a wrong "public" call is a disclosure.
</context>

## Always do

<required>

- **Name the sink before writing to it.** Before any `log`, `print`, `console`, response body,
  analytics event, or persisted record, state what is going in. Whole objects are the failure mode:
  `log.info(user)` leaks every field the model was added later.
- **Redact at the boundary, not at the source.** The logger, serializer, or error mapper is where
  redaction belongs, so a new caller inherits it. A redaction applied at one call site is one call
  site.
- **Keep server secrets server-side.** A build-time public env prefix (`NEXT_PUBLIC_`, `VITE_`,
  `EXPO_PUBLIC_`, `REACT_APP_`, `PUBLIC_`, and their equivalents) inlines the value into the shipped
  bundle. Anything behind such a prefix is public. Verify the prefix rule for the detected build tool
  rather than assuming.
- **Give errors two shapes.** One for the caller with a stable code and a safe message, one for the
  operator with the detail, correlated by an opaque request ID. Never let the second shape reach the
  first audience.
- **Store client-side by capability, not by convenience.** Auth material belongs in the platform's
  protected store or an httpOnly cookie, not in web local or session storage and not in mobile
  plaintext preference files.
</required>

## Ask first

<escalation>

- Adding a new sink for user data: a log destination, an analytics provider, a crash reporter, a
  session-replay tool, or a third-party SDK on a client surface.
- Widening a DTO, response body, or event payload with a field that was not previously exposed.
- Relaxing an existing redaction, filter, or deny rule.
- Committing any fixture, snapshot, or recorded response captured from a real environment.
</escalation>

## Never do

<required>

- Never write a credential-shaped literal into source, test, fixture, config, or commit message.
  Use a placeholder plus the configured secret source.
- Never log authentication material or raw personal data, including inside a caught exception.
- Never return an internal error, stack trace, or driver message to a client.
- Never put sensitive values in a URL path or query string; they land in server logs, proxy logs,
  browser history, and referrer headers.
- Never disable or narrow a leak guard to make an edit pass. Fix the value or mark an explicit,
  reasoned exception.
</required>

## Applying this during each command

<rules>

| Command | The constraint it must add |
|---|---|
| [`/spec`](../../commands/spec.md) | Name the sensitive data classes the feature touches, and where each is allowed to appear |
| [`/plan`](../../commands/plan.md) | Any task touching auth, user data, logging, or a client surface carries a redaction acceptance criterion |
| [`/build`](../../commands/build.md) | Before commit, re-read the diff for the five "always do" items above |
| [`/test`](../../commands/test.md) | A boundary that must redact gets a test that fails when redaction is removed |
| [`/code-simplify`](../../commands/code-simplify.md) | A refactor may not widen what a sink receives. Behavior-preserving is not disclosure-preserving |
| [`/review`](../../commands/review.md) | Check the sinks the diff added or changed, not only the logic |
| [`/goal`](../../commands/goal.md) | Redact before writing evidence under `tmp/` |

`/code-simplify` is the one most often missed. Tests stay green while a widened DTO, an inlined
error object, or a moved log line creates a disclosure, because no test asserts what a sink must
*not* receive.
</rules>

## Relation to the deterministic guard

<context>

[`hooks/sensitive-data-guard.py`](../../hooks/sensitive-data-guard.py) is the sensor for the subset
a regular expression can settle. It is a floor, not the ceiling:

- It cannot tell whether `user` in `log.info(user)` carries personal data.
- It cannot tell whether a response field is authorized for its audience.
- It cannot see a leak assembled across two files.

A clean guard run is not evidence that this skill was applied.
[`references/tool-safety-and-permissions.md`](../../references/tool-safety-and-permissions.md)
states the matching rule for permissions: never rely on model judgment alone to avoid secrets. The
converse also holds. Never rely on the sensor alone.
</context>

## Verification

<verification>

- [ ] Every sink added or changed by the diff was named, and its payload is known field by field
- [ ] No credential-shaped literal in the diff, including tests and fixtures
- [ ] Client-visible errors carry a code and a safe message, with detail behind a request ID
- [ ] No secret sits behind a build-time public env prefix
- [ ] Auth material is in a protected store, not web local storage or plaintext mobile preferences
- [ ] Redaction lives at the boundary and a new caller inherits it
- [ ] Guard exceptions, if any, are marked inline with a stated reason
</verification>

## Why

The toolkit's security material is deep but user-invoked, so it arrives at review time, after the
code exists. Disclosure is not a defect that review reliably catches, because the code works. This
skill moves the constraint to the moment of writing.

## Routing & discovery

- Use when implementing, refactoring, testing, or reviewing anything that reads or emits user data.
- Do not use as a replacement for [`security-and-hardening`](../security-and-hardening/SKILL.md)
  when the task is authn/authz design, injection, or dependency risk.

Invoke by default for any work touching auth, user data, logging, error handling, configuration, or
a client surface. Skip only for changes with no sink and no data, such as pure formatting.

## Permissions & authority

- **Tools:** Read, Grep, Glob, Edit; Shell only for repo-documented lint and test commands.
- **Paths:** Follow [`.ai-agents/PERMISSIONS.md`](../../PERMISSIONS.md). Never read credential or
  secret material; this skill governs what code writes, and never needs a real secret to do it.
- **Cursor:** reinforce via `.cursor/rules` and the repository charter.
