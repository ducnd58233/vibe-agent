"""Inject local-first precedence and routing reminders on asset-authoring prompts.

Runs on UserPromptSubmit. Fires only when the prompt looks like asset authoring or
routing work, so ordinary prompts stay free of extra context.

Reads the Claude Code payload on stdin (`prompt`) and returns
`hookSpecificOutput.additionalContext`, which UserPromptSubmit surfaces alongside
the prompt. Always exits 0 — this hook informs, it never blocks.
"""

from __future__ import annotations

import json
import re
import sys

# Prompt intents that benefit from the reminder. Kept narrow on purpose.
# Verbs cover English and Vietnamese, since prompts in this repo arrive in both.
ASSET_NOUN = re.compile(
    r"\b(?:skill|subagent|command|hook|reference|stack[- ]profile|router|template)s?\b",
    re.IGNORECASE,
)

ACTION_VERB = re.compile(
    r"\b(?:"
    r"new|add|create|author|write|update|rename|delete|remove|edit|refactor"
    r"|t[aạ]o|th[eê]m|s[uử]a|vi[eế]t|x[oó]a|xo[aá]|c[aậ]p\s*nh[aậ]t|đ[oổ]i\s*t[eê]n|m[oớ]i"
    r")\b",
    re.IGNORECASE | re.UNICODE,
)

ROUTING_PATTERN = re.compile(
    r"\b(?:which|what|n[aà]o)\b[^.\n]{0,40}\b(?:skill|agent|command|profile|asset)\b"
    r"|\brout(?:e|ing)\b",
    re.IGNORECASE | re.UNICODE,
)


def _should_remind(prompt: str) -> bool:
    if ROUTING_PATTERN.search(prompt):
        return True
    return bool(ASSET_NOUN.search(prompt) and ACTION_VERB.search(prompt))

REMINDER = (
    "Repository reminders for this request:\n"
    "1. Local-first precedence - check the workspace root for its own rules and "
    "templates (AGENTS.md, CLAUDE.md, CLAUDE.local.md, .cursor/rules/, its own "
    "TEMPLATE.md, existing file patterns) before applying any toolkit default. "
    "On conflict follow the local rule and state the divergence.\n"
    "2. Routing - read .ai-agents/ROUTER.md, then the matching folder ROUTER.md, "
    "before selecting an asset.\n"
    "3. Authoring - follow that folder's TEMPLATE.md and complete every required "
    "section.\n"
    "4. Router tables - after adding, renaming, or removing an asset, update that "
    "folder's ROUTER.md in the same change."
)


def _read_payload() -> dict[str, object]:
    # Decode explicitly: stdin defaults to the locale encoding (cp1252 on Windows),
    # which mangles non-ASCII prompt text before the patterns ever see it.
    raw = sys.stdin.buffer.read().decode("utf-8", errors="replace").strip()
    if not raw:
        return {}
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return payload if isinstance(payload, dict) else {}


def main() -> int:
    payload = _read_payload()
    prompt = payload.get("prompt")
    if not isinstance(prompt, str) or not prompt.strip():
        return 0

    if not _should_remind(prompt):
        return 0

    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "UserPromptSubmit",
            "additionalContext": REMINDER,
        },
    }))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
