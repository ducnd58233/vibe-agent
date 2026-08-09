"""Warn when an edit puts sensitive data somewhere an end user can read it.

Tier 1 of a two-tier design. This script owns the *soft* signals: patterns that are
usually a leak but have real legitimate uses, so a refusal here would block honest
work. It always exits 0 and reports through the JSON channel, like design-token-guard
and ui-slop-guard.

Tier 2 is the runtime (`vibe-agent hook pre-tool-use`), which owns the *hard* signals:
a literal private-key block, a provider token with a recognizable prefix, an AWS secret
key. Those have no legitimate reason to enter source, so they are refused rather than
warned about. Nothing in this file duplicates that set.

The split matters for false positives. `NEXT_PUBLIC_API_KEY` is a leak when it holds a
server secret and correct when it holds a publishable key, and no regular expression can
tell which. That is a warning. `-----BEGIN RSA PRIVATE KEY-----` needs no such judgement.

Named `sensitive-data-guard`, not `secret-leak-guard`, for a permissions reason worth
keeping: `.claude/settings.json` denies `Read(**/*secret*)`, deny overrides allow, and a
file whose own name matches that pattern cannot be read or edited by the agent
maintaining it. PERMISSIONS.md records the same collision for an earlier `**/*token*`
rule against design-token-guard.py.

Reads the Claude Code PostToolUse payload on stdin (`tool_name`, `tool_input.file_path`)
and the Cursor afterFileEdit payload (`file_path` at top level).

Opt out per line by adding a `sensitive-data-guard: allow` comment on that line, with a
reason. An unexplained opt-out is the thing a reviewer should ask about.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

SOURCE_SUFFIXES = {
    # Web and cross-platform clients
    ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
    ".vue", ".svelte", ".astro", ".html",
    # Mobile and native
    ".swift", ".kt", ".kts", ".java", ".dart", ".m", ".mm",
    # Backend
    ".py", ".go", ".rs", ".rb", ".php", ".cs", ".scala", ".ex", ".exs",
    # Config that carries build-time env values
    ".json", ".yaml", ".yml", ".toml",
}

ALLOW_MARKER = "sensitive-data-guard: allow"

# This guard and its contract test. See _target_file for why these two, and only
# these two, are exempt.
SELF_EXEMPT = {"sensitive-data-guard.py", "sensitive-data-guard-test.py"}

COMMENT_PREFIXES = ("//", "#", "*", "/*", "--", ";", "<!--")

# A line that already redacts is not a finding. Checked before every pattern so a
# correct implementation stays quiet.
REDACTION_MARKERS = re.compile(
    r"(?i)(redact|scrub|sanitiz|mask(?:ed|ing)?\b|\[REDACTED\]|\*{3,}|omit(?:ted)?\b)"
)

# Values that read as a placeholder rather than a real credential.
PLACEHOLDER = re.compile(
    r"(?i)(your[-_ ]|example|placeholder|changeme|change[-_]me|dummy|fake|sample|"
    r"xxx+|\.\.\.|<[^>]+>|\{\{|\$\{|process\.env|os\.environ|getenv|import\.meta\.env|"
    r"System\.getenv|ENV\[)"
)

# Shared vocabulary. Kept as fragments so every check reads the same way.
# Do not call .upper() on these when composing: uppercasing a pattern turns \w
# into \W and inverts the class. Every check passes re.IGNORECASE instead.
SECRET_NAME = (
    r"(?:pass(?:wo?rd|wd)|secret|token|api[_-]?key|apikey|credential|private[_-]?key|"
    r"auth(?:oriz\w*|entic\w*)?|jwt|bearer|session[_-]?id|access[_-]?key)"
)
PII_NAME = (
    r"(?:e[-_]?mail|phone[_-]?(?:number)?|ssn|social[_-]?security|date[_-]?of[_-]?birth|"
    r"birth[_-]?date|\bdob\b|home[_-]?address|passport|national[_-]?id|credit[_-]?card|"
    r"card[_-]?number|\bcvv\b|\biban\b|tax[_-]?id)"
)
# Objects that carry unknown fields. The leak is that a field added later rides along.
BULK_OBJECT = (
    r"(?:user|account|profile|session|customer|member|patient|employee|payload|"
    r"req(?:uest)?|body|params|claims|creds?)"
)

# Any call that writes to a console, a log, or a device log across the languages above.
LOG_CALL = (
    r"(?:console\.(?:log|info|warn|error|debug|trace|dir|table)|"
    r"log(?:ger|ging)?\.(?:log|info|warn|warning|error|debug|trace|fatal|critical|exception)|"
    r"print(?:ln|f)?|fmt\.(?:Print|Sprint)\w*|System\.out\.print\w*|"
    r"NSLog|os_log|Log\.[dviwe]|Logger\.\w+|slog\.\w+|debugPrint|dump)"
)

# Build-time prefixes that inline a value into the shipped client bundle.
PUBLIC_ENV_PREFIX = (
    r"(?:NEXT_PUBLIC_|VITE_|EXPO_PUBLIC_|REACT_APP_|NUXT_PUBLIC_|GATSBY_|"
    r"PUBLIC_|VUE_APP_|NG_APP_)"
)

# (id, compiled pattern, message)
LINE_CHECKS = [
    (
        "public-env-secret",
        re.compile(rf"{PUBLIC_ENV_PREFIX}[A-Z0-9_]*{SECRET_NAME}", re.IGNORECASE),
        "A build-time public env prefix inlines this value into the client bundle, where "
        "any user can read it. If this is a server secret, drop the prefix and read it "
        "server-side. If it is genuinely publishable, mark the line.",
    ),
    (
        "credential-in-log",
        re.compile(rf"{LOG_CALL}\s*\([^)]*{SECRET_NAME}", re.IGNORECASE),
        "This log call receives authentication material. Log an identifier or a hash "
        "instead, and redact in the logger so new callers inherit it.",
    ),
    (
        "pii-in-log",
        re.compile(rf"{LOG_CALL}\s*\([^)]*{PII_NAME}", re.IGNORECASE),
        "This log call receives personal data. Log an opaque user ID and correlate "
        "out of band.",
    ),
    (
        "bulk-object-in-log",
        re.compile(rf"{LOG_CALL}\s*\(\s*{BULK_OBJECT}\s*[,)]", re.IGNORECASE),
        "Logging a whole object publishes every field it has now and every field added "
        "later. Name the fields you need.",
    ),
    (
        "auth-in-web-storage",
        re.compile(
            rf"(?:local|session)Storage\.setItem\s*\(\s*[\"'`]?[\w.\-]*{SECRET_NAME}",
            re.IGNORECASE,
        ),
        "Web storage is readable by any script on the origin, so an XSS becomes a token "
        "theft. Use an httpOnly cookie, or keep the token in memory.",
    ),
    (
        "auth-in-mobile-plaintext",
        re.compile(
            rf"(?:AsyncStorage\.setItem|SharedPreferences|getSharedPreferences|"
            rf"UserDefaults(?:\.standard)?\.set|SecurePreferences?\.putString)"
            rf"[^)\n]*{SECRET_NAME}",
            re.IGNORECASE,
        ),
        "This store is plaintext on a rooted or jailbroken device and often lands in "
        "device backups. Use Keychain, Keystore, or the platform secure store.",
    ),
    (
        "sensitive-in-url",
        re.compile(
            rf"[?&]{SECRET_NAME}\s*=|"
            rf"(?:searchParams\.(?:set|append)|URLSearchParams)\s*\([^)]*{SECRET_NAME}",
            re.IGNORECASE,
        ),
        "Values in a URL land in server logs, proxy logs, browser history, and the "
        "referrer header. Move it to a header or a request body.",
    ),
    (
        "stack-trace-to-client",
        re.compile(
            r"(?:res|response|reply|ctx)\.(?:json|send|status\(\d+\)\.\w+)\s*\([^)]*"
            r"(?:\.stack\b|\berr(?:or)?\.message\b|traceback|getStackTrace|"
            r"printStackTrace|\bexc?\.__\w+__)",
            re.IGNORECASE,
        ),
        "This returns internal error detail to the caller. Send a stable code plus a "
        "safe message, and keep the detail server-side behind a request ID.",
    ),
    (
        "hardcoded-credential",
        re.compile(
            rf"\b\w*{SECRET_NAME}\w*\s*[:=]\s*[\"'`]([^\"'`\n]{{8,}})[\"'`]",
            re.IGNORECASE,
        ),
        "A credential-shaped literal belongs in the configured secret source, not in "
        "source control. Anyone with repository access can read it, and rotating it "
        "means a commit.",
    ),
]


def _read_payload() -> dict[str, object]:
    # Decode explicitly: stdin defaults to the locale encoding (cp1252 on Windows),
    # which corrupts non-ASCII file paths before they are resolved.
    raw = sys.stdin.buffer.read().decode("utf-8", errors="replace").strip()
    if not raw:
        return {}
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return payload if isinstance(payload, dict) else {}


def _target_file(payload: dict[str, object]) -> Path | None:
    # Claude PostToolUse fires for every tool, so the tool name has to be checked.
    # Cursor afterFileEdit is already edit-only and sends no tool_name, so its
    # absence is not a reason to bail out.
    tool_name = payload.get("tool_name")
    if tool_name is not None and tool_name not in {"Edit", "Write", "NotebookEdit"}:
        return None

    file_path_value: object = None
    tool_input = payload.get("tool_input")
    if isinstance(tool_input, dict):
        for key in ("file_path", "path"):
            candidate = tool_input.get(key)
            if isinstance(candidate, str) and candidate:
                file_path_value = candidate
                break
    if not isinstance(file_path_value, str) or not file_path_value:
        for key in ("file_path", "path"):
            candidate = payload.get(key)
            if isinstance(candidate, str) and candidate:
                file_path_value = candidate
                break
    if not isinstance(file_path_value, str) or not file_path_value:
        return None

    path = Path(file_path_value)
    if path.name in SELF_EXEMPT:
        # A detector and its fixtures necessarily contain the shapes they detect,
        # and the per-line opt-out cannot help here: the marker is what makes a
        # line skippable, so applying it to a "must flag" fixture would neuter the
        # case under test. Two exact filenames, not a category. Test files in
        # general are still scanned, because a credential in a real fixture is a
        # real leak.
        return None
    if path.suffix.lower() not in SOURCE_SUFFIXES:
        return None
    return path if path.is_file() else None


def _is_skippable(line: str) -> bool:
    """A comment, an opt-out, or a line that already redacts is not a finding."""
    stripped = line.strip()
    if not stripped:
        return True
    if ALLOW_MARKER in line:
        return True
    if stripped.startswith(COMMENT_PREFIXES):
        return True
    return bool(REDACTION_MARKERS.search(line))


def scan(text: str) -> list[str]:
    """Return one finding string per flagged line. Public so the contract test can call it."""
    findings: list[str] = []

    for number, line in enumerate(text.splitlines(), start=1):
        if _is_skippable(line):
            continue
        for check_id, pattern, message in LINE_CHECKS:
            match = pattern.search(line)
            if not match:
                continue
            # The hardcoded-credential check is the noisiest, because tests and
            # examples legitimately assign credential-shaped names. Only a literal
            # that does not read as a placeholder is worth reporting.
            if check_id == "hardcoded-credential":
                if PLACEHOLDER.search(match.group(1)) or PLACEHOLDER.search(line):
                    continue
            findings.append(f"L{number} [{check_id}] {message}")

    return findings


def main() -> int:
    payload = _read_payload()
    path = _target_file(payload)
    if path is None:
        return 0

    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return 0

    findings = scan(text)
    if not findings:
        return 0

    detail = "\n".join(f"  - {item}" for item in findings)
    report = (
        f"sensitive-data-guard flagged {len(findings)} possible disclosure(s) in {path.name}:\n"
        f"{detail}\n"
        "Apply secure-by-default: name the sink, redact at the boundary, keep server "
        f"secrets out of client builds. If a line is deliberate, mark it with a "
        f"`{ALLOW_MARKER}` comment and state why."
    )

    # Both fields on purpose. systemMessage reaches the person; additionalContext
    # reaches the model, which is the one that has to fix the file. A warning only
    # the human can see is a warning the agent will repeat on the next edit.
    print(
        json.dumps(
            {
                "systemMessage": report,
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": report,
                },
            }
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
