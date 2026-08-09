# Sensitive Data Exposure

<context>

Organized by **leak channel**, not by vulnerability class. The question a channel answers is the
one an agent actually faces mid-edit: *this value is about to go somewhere, can the wrong person
read it there?*

Companion to [`security-checklist.md`](security-checklist.md), which is organized by OWASP category
and answers a different question. Use the always-on
[`secure-by-default`](../skills/secure-by-default/SKILL.md) skill for the write-time rules, and
[`security-and-hardening`](../skills/security-and-hardening/SKILL.md) for authn/authz and injection
depth.

**Scope split:** this file names **channels and rules**, which generalize. Concrete mechanisms
(which store, which flag, which library) belong in [`stack-profiles/`](../stack-profiles/ROUTER.md),
per the tool-agnostic rule in root [`AGENTS.md`](../../AGENTS.md).
</context>

## The audience test

<rules>

Before any value leaves your control, name who can read it at the destination:

| Destination | Who can actually read it |
|---|---|
| Client bundle | Every user, every outside developer, every scanner. Permanently, once shipped |
| Browser storage | Any script on the origin, so any XSS |
| Mobile plaintext store | Anyone with the device unlocked, rooted, or backed up |
| Console / device log | The user, anyone with the device, any app reading device logs on older platforms |
| Rendered UI or DOM | The user, plus every screenshot, screen recording, and session-replay tool |
| API response | The caller, plus any proxy and any client-side logger |
| Server log | Every operator, every log aggregator, every support engineer, every breach of that aggregator |
| Telemetry / analytics | The vendor, and everyone with vendor access |
| Build artifact / CI log | Everyone with repository or pipeline read access |

"Only our developers see it" is not a control. Developers leave, logs get exported, and an attacker
who reaches any of these reaches all of them.
</rules>

## Channel 1: the client bundle

<context>

Anything the build inlines is public forever. Rotating it means a redeploy, and the old bundle is
already cached.

- Values behind a build-time public prefix are inlined verbatim. The prefix differs per tool
  (`NEXT_PUBLIC_`, `VITE_`, `EXPO_PUBLIC_`, `REACT_APP_`, `PUBLIC_`, and others). Detect the rule
  from the build tool the repo actually uses; do not assume.
- A "publishable" or "anon" key from a vendor is designed for this. A "secret", "service", or
  "admin" key from the same vendor is not, and the two often differ by one word.
- Source maps expose original source, comments, and sometimes inlined config. Decide deliberately
  whether they ship, and if they do, whether they are restricted to an error-reporting vendor.
- Mobile and embedded binaries are extractable. A key compiled into an app is a published key;
  string obfuscation delays extraction, it does not prevent it.
- Server-only code must be provably unreachable from the client entry point, not merely
  conventionally separated.

**Test:** build for production, then search the output directory for the value. If it appears, it
is public.
</context>

## Channel 2: client-side storage

<rules>

- Web: browser local and session storage are readable by any script on the origin. An XSS becomes
  a token theft with no further work. Prefer an httpOnly, secure, sameSite cookie, or hold the
  token in memory and accept re-auth on reload.
- Mobile: plaintext preference stores and unencrypted files are readable on a rooted or jailbroken
  device and often ride along in device backups. Use the platform secure store. Mark sensitive
  files as excluded from backup where the platform supports it.
- Embedded: unencrypted flash is readable with physical access. Per-device keys beat a shared
  firmware key, because one extraction should not compromise the fleet.
- Anything cached offline is storage too, including service-worker caches and offline databases.
- On logout, clear it. A token that outlives the session is a token someone else can use.

## Channel 3: consoles and device logs

- A debug statement that reaches production is a disclosure. Strip it in the production build, or
  route it through a logger that is silent there.
- Whole-object logging is the recurring failure. `console.log(user)` publishes every field the
  object has now and every field a teammate adds next quarter. Name the fields.
- Device logs are shared surfaces: Android logcat, iOS unified logging, and a serial console on
  embedded hardware. Treat them as user-visible.
- Framework and driver errors often carry the query, the payload, or the connection string. Catch
  and re-shape them before they reach any log you do not control.

## Channel 4: rendered UI and the DOM

- Data can leak without being displayed. Hidden fields, `data-` attributes, unrendered JSON
  payloads embedded for hydration, and HTML comments are all readable in view-source.
- Fetching more than you render leaks the remainder. Filter server-side, not in the component.
- Error boundaries and error pages must not print stack traces, file paths, or query text.
- Mask by default in the UI where the full value is rarely needed: partial account numbers,
  partial contact details, no full government IDs.
- Screenshots, screen recordings, and session-replay tools capture whatever is on screen.
  Mark sensitive fields excluded, and verify the exclusion actually applies rather than trusting
  the setting.
- Autofill, spellcheck, and third-party keyboards can forward field contents. Disable them on
  credential and high-sensitivity fields.

## Channel 5: API response shape

- Build responses from an explicit output type, never by serializing the storage record. An
  allowlist stays correct when a column is added; a denylist does not.
- Errors get two shapes: a stable code and a safe message for the caller, full detail server-side,
  joined by an opaque request ID the user can quote to support.
- Do not vary an error between "no such account" and "wrong password"; the difference enumerates
  accounts. The same applies to timing on authentication paths.
- Identifiers in responses should not reveal volume or allow traversal to another tenant's row.
- Verify authorization per record, not per endpoint. A list endpoint that filters by a
  client-supplied tenant ID is an access-control bug wearing a filter.

## Channel 6: server logs and telemetry

- Redact in the logger, serializer, or error mapper. Redaction at a call site protects that call
  site only; a new caller added later inherits nothing.
- Never log authentication material, even inside a caught exception. Exception objects routinely
  carry the arguments that caused them.
- Log an opaque user identifier, and correlate to personal data out of band through a system with
  its own access control.
- Request and response body logging is the most common accidental dump. If it must exist, make it
  field-allowlisted and off by default.
- Trace attributes, span names, metric labels, and error-reporting context are all logs with a
  different name. The same rules apply.
- Set retention deliberately. Data you no longer hold cannot leak.
</rules>

## Channel 7: build artifacts, CI, and agent evidence

<context>

- Secrets passed as build arguments persist in image layers and build history, even when the final
  stage does not reference them.
- CI logs are readable by everyone with pipeline access, and often by everyone with repository
  access. Masking depends on the platform recognizing the value; a transformed secret is not
  masked.
- Committed history is permanent. A secret removed in a later commit is still in the history, and
  rotation is the only real fix.
- Agent evidence under `tmp/` may contain PR comments, test output, and captured responses. Redact
  before writing, per [`goal-verification-records.md`](goal-verification-records.md).
</context>

## Verification

<verification>

Channel-by-channel, this is what an agent should actually run or check:

- [ ] Production build searched for each configured secret value; none present
- [ ] No auth material in browser storage or a plaintext mobile store
- [ ] Production build emits no debug console output
- [ ] View-source of a data-bearing page checked for unrendered payloads and hidden fields
- [ ] Every API response built from an explicit output type
- [ ] Client-facing errors carry a code plus a safe message; detail is behind a request ID
- [ ] Logger redaction covers auth material and personal data at the boundary
- [ ] A test exists that fails when a redaction is removed
- [ ] CI logs and image layers checked for secret values
- [ ] `tmp/` evidence redacted before write

The deterministic part of this list is enforced by
[`hooks/sensitive-data-guard.py`](../hooks/sensitive-data-guard.py) on `Edit`/`Write`. The guard is
a floor; it cannot tell whether an object carries personal data, whether a response field is
authorized for its audience, or whether a leak is assembled across two files.
</verification>

## Standards

<rules>

Mapped against **OWASP Top 10:2025**, **OWASP ASVS 5.0.0** (released 30 May 2025), and **CWE**.
Versions are pinned here rather than inline, because category numbering changes between editions
while the rules above do not. When you bump a version, this table is the only thing to re-check.

| Channel | OWASP Top 10:2025 | CWE |
|---|---|---|
| 1. Client bundle | A02 Security Misconfiguration | CWE-798 Use of Hard-coded Credentials, CWE-200 |
| 2. Client-side storage | A04 Cryptographic Failures | CWE-922 Insecure Storage of Sensitive Information, CWE-312 |
| 3. Consoles and device logs | A09 Security Logging and Alerting Failures | CWE-532 Insertion of Sensitive Information into Log File |
| 4. Rendered UI and DOM | A01 Broken Access Control | CWE-200 Exposure of Sensitive Information, CWE-359 |
| 5. API response shape | A01 Broken Access Control, A10 Mishandling of Exceptional Conditions | CWE-209 Generation of Error Message Containing Sensitive Information, CWE-213 |
| 6. Server logs and telemetry | A09 Security Logging and Alerting Failures | CWE-532, CWE-359 Exposure of Private Personal Information |
| 7. Build artifacts and CI | A03 Software Supply Chain Failures | CWE-798, CWE-538 |

Two notes on currency, verified against the OWASP project pages:

- The 2025 edition dropped "Vulnerable and Outdated Components" and "Server-Side Request Forgery"
  as standalone categories, and added A03 Software Supply Chain Failures and A10 Mishandling of
  Exceptional Conditions. The unlabeled table in
  [`security-checklist.md`](security-checklist.md) still reflects the 2021 edition.
- Mobile-specific verification maps to OWASP MASVS, which is platform-scoped. Keep those mappings
  in the mobile [`stack-profiles/`](../stack-profiles/ROUTER.md) rather than here, so this file
  stays stack-neutral.
</rules>

## Related

<references>

- [`secure-by-default`](../skills/secure-by-default/SKILL.md) - the always-on write-time rules
- [`security-checklist.md`](security-checklist.md) - OWASP-category view
- [`security-and-hardening`](../skills/security-and-hardening/SKILL.md) - authn/authz, injection, dependencies
- [`tool-safety-and-permissions.md`](tool-safety-and-permissions.md) - agent tool and path boundaries
- [`goal-verification-records.md`](goal-verification-records.md) - redaction of `tmp/` evidence
</references>
