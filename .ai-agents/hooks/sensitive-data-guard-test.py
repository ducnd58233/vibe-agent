"""Contract test for sensitive-data-guard.py.

Run: python3 .ai-agents/hooks/sensitive-data-guard-test.py

Half the cases are false-positive guards. That ratio is deliberate: a warn-only guard
that cries wolf gets muted, and a muted guard protects nothing. Every case here that
expects silence is a line real code contains.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

# The guard's filename has a hyphen, so it cannot be imported by name and is
# loaded by path instead. Bytecode is disabled first: the sibling test shells out
# and leaves nothing behind, and a __pycache__ directory appearing in the hooks
# folder after running a test is noise no one asked for.
sys.dont_write_bytecode = True

MODULE_PATH = Path(__file__).with_name("sensitive-data-guard.py")
spec = importlib.util.spec_from_file_location("sensitive_data_guard", MODULE_PATH)
if spec is None or spec.loader is None:
    print(f"ACCESS-FAILED: {MODULE_PATH}")
    raise SystemExit(1)
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)

# (name, source line, expected check id or None for "must stay silent")
CASES: list[tuple[str, str, str | None]] = [
    # --- must flag ---
    (
        "server secret behind a public build prefix",
        'const k = process.env.NEXT_PUBLIC_STRIPE_SECRET_KEY;',
        "public-env-secret",
    ),
    (
        "vite prefix carrying a token",
        'const t = import.meta.env.VITE_AUTH_TOKEN;',
        "public-env-secret",
    ),
    (
        "token passed to console",
        'console.log("session", accessToken);',
        "credential-in-log",
    ),
    (
        "python logger receiving a password",
        'logger.debug("login attempt %s", password)',
        "credential-in-log",
    ),
    (
        "go printf receiving personal data",
        'fmt.Printf("contacting %s", customerEmail)',
        "pii-in-log",
    ),
    (
        "whole user object logged",
        'console.log(user);',
        "bulk-object-in-log",
    ),
    (
        "auth token in web storage",
        "localStorage.setItem('auth_token', token);",
        "auth-in-web-storage",
    ),
    (
        "ios plaintext defaults holding a token",
        'UserDefaults.standard.set(authToken, forKey: "session")',
        "auth-in-mobile-plaintext",
    ),
    (
        "api key in a query string",
        'const r = await fetch(`/v1/items?api_key=${key}`);',
        "sensitive-in-url",
    ),
    (
        "stack trace returned to caller",
        'res.status(500).json({ error: err.stack });',
        "stack-trace-to-client",
    ),
    (
        "hardcoded password literal",
        'const password = "hX7!qpZm2Lr9";',
        "hardcoded-credential",
    ),
    # --- must stay silent ---
    (
        "commented-out credential",
        '// const password = "hX7!qpZm2Lr9";',
        None,
    ),
    (
        "python comment mentioning a token",
        '# token = "abc123def456"',
        None,
    ),
    (
        "explicit inline opt-out",
        'const password = "hX7!qpZm2Lr9"; // sensitive-data-guard: allow rotated test fixture',
        None,
    ),
    (
        "logger that already redacts",
        'logger.info("password redacted for user %s", user_id)',
        None,
    ),
    (
        "masking helper on personal data",
        'const shown = maskEmail(user.email);',
        None,
    ),
    (
        "placeholder credential in an example",
        'const apiKey = "your-api-key-here";',
        None,
    ),
    (
        "credential read from the environment",
        'const apiKey = process.env.STRIPE_SECRET_KEY;',
        None,
    ),
    (
        "public build prefix on a non-secret",
        'const url = process.env.NEXT_PUBLIC_APP_URL;',
        None,
    ),
    (
        "logging a harmless scalar",
        'console.log(itemCount);',
        None,
    ),
    (
        "log message whose text mentions a user",
        'logger.info("user created", { id: user.id });',
        None,
    ),
    (
        "non-sensitive query parameter",
        'const r = await fetch(`/v1/items?page=${page}`);',
        None,
    ),
    (
        "safe error shape returned to caller",
        "res.status(500).json({ code: 'INTERNAL', requestId });",
        None,
    ),
    (
        "short value under the credential length floor",
        'const token = "abc";',
        None,
    ),
]


def run() -> int:
    failures: list[str] = []

    for name, line, expected in CASES:
        findings = guard.scan(line)
        matched_ids = {item.split("[", 1)[1].split("]", 1)[0] for item in findings if "[" in item}

        if expected is None:
            if findings:
                failures.append(
                    f"FALSE POSITIVE  {name}\n"
                    f"    line: {line}\n"
                    f"    flagged as: {', '.join(sorted(matched_ids))}"
                )
        else:
            if expected not in matched_ids:
                failures.append(
                    f"MISSED          {name}\n"
                    f"    line: {line}\n"
                    f"    expected: {expected}\n"
                    f"    got: {', '.join(sorted(matched_ids)) or 'nothing'}"
                )

    total = len(CASES)
    silent = sum(1 for _, _, expected in CASES if expected is None)
    print(f"sensitive-data-guard: {total} cases ({silent} false-positive guards)")

    if failures:
        print(f"\n{len(failures)} failing:\n")
        for failure in failures:
            print(failure)
            print()
        return 1

    print("all pass")
    return 0


if __name__ == "__main__":
    raise SystemExit(run())
