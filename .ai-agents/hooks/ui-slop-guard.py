"""Warn on generic AI-aesthetic signatures in UI files after Edit/Write.

Binary pattern checks only - no model calls. Complements design-token-guard.py,
which owns raw-color detection; this script owns the remaining slop signatures.

Reads the Claude Code PostToolUse payload on stdin (`tool_name`, `tool_input.file_path`)
and always exits 0, so it warns without blocking the edit.

Opt out per line by adding a `ui-slop-guard: allow` comment on that line.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

UI_SUFFIXES = {
    ".tsx", ".jsx", ".ts", ".js", ".mjs",
    ".vue", ".svelte", ".astro", ".html",
    ".css", ".scss", ".sass", ".less",
}

ALLOW_MARKER = "ui-slop-guard: allow"

# Density checks only fire past a threshold, so one deliberate use stays quiet.
RADIUS_THRESHOLD = 4
SHADOW_THRESHOLD = 4

SLOP_HUES = r"(?:indigo|purple|violet|fuchsia)"

# (id, compiled pattern, message) - each match is reported with its line number.
LINE_CHECKS = [
    (
        "slop-gradient",
        re.compile(rf"bg-(?:gradient|linear)-to-[a-z]+\b.*\b(?:from|via|to)-{SLOP_HUES}-\d{{2,3}}"),
        "Indigo/purple gradient is the most recognizable AI-generated default. "
        "Use a brand hue from the design system, or drop the gradient.",
    ),
    (
        "slop-gradient-css",
        re.compile(r"linear-gradient\([^)]*(?:#6366f1|#818cf8|#8b5cf6|#a855f7|#c084fc)", re.IGNORECASE),
        "Hard-coded indigo/purple gradient stop. Reference a semantic token instead.",
    ),
    (
        "arbitrary-value",
        re.compile(r"(?<![\w-])(?:[a-z]+-)+\[[^\]\s]+\]"),
        "Arbitrary Tailwind value escapes the spacing/color scale. "
        "Use a scale step or add a token if the value is genuinely new.",
    ),
    (
        "default-font-stack",
        re.compile(r"font-family:\s*[\"']?(?:Inter|Roboto)\b", re.IGNORECASE),
        "Inter/Roboto as the lead face is an AI-default type choice. "
        "Pick the project's face, or a deliberate pairing.",
    ),
]

# (id, compiled pattern, threshold, message) - reported once per file when count >= threshold.
DENSITY_CHECKS = [
    (
        "radius-monotony",
        re.compile(r"\brounded-(?:2xl|3xl)\b"),
        RADIUS_THRESHOLD,
        "Uniform large corner radius flattens hierarchy. "
        "Vary radius by surface level as the design system defines.",
    ),
    (
        "shadow-stacking",
        re.compile(r"\bshadow-(?:xl|2xl)\b"),
        SHADOW_THRESHOLD,
        "Heavy layered shadows compete with content. "
        "Prefer one subtle elevation step, or none.",
    ),
]


def _read_payload() -> dict[str, object]:
    raw = sys.stdin.read().strip()
    if not raw:
        return {}
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return payload if isinstance(payload, dict) else {}


def _target_file(payload: dict[str, object]) -> Path | None:
    if payload.get("tool_name") not in {"Edit", "Write"}:
        return None
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict):
        return None
    file_path_value = tool_input.get("file_path")
    if not isinstance(file_path_value, str) or not file_path_value:
        return None

    path = Path(file_path_value)
    if path.suffix.lower() not in UI_SUFFIXES:
        return None
    return path if path.is_file() else None


def _scan(text: str) -> list[str]:
    lines = text.splitlines()
    findings: list[str] = []

    for check_id, pattern, message in LINE_CHECKS:
        for number, line in enumerate(lines, start=1):
            if ALLOW_MARKER in line:
                continue
            if pattern.search(line):
                findings.append(f"L{number} [{check_id}] {message}")

    for check_id, pattern, threshold, message in DENSITY_CHECKS:
        count = len(pattern.findall(text))
        if count >= threshold:
            findings.append(f"[{check_id}] {count} occurrences. {message}")

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

    findings = _scan(text)
    if not findings:
        return 0

    detail = "\n".join(f"  - {item}" for item in findings)
    report = (
        f"ui-slop-guard flagged {len(findings)} AI-aesthetic signature(s) in {path.name}:\n"
        f"{detail}\n"
        "Fix them against the project's design registry, or mark a deliberate "
        f"exception with a `{ALLOW_MARKER}` comment on the line."
    )

    print(json.dumps({
        "systemMessage": report,
        "hookSpecificOutput": {
            "hookEventName": "PostToolUse",
            "additionalContext": report,
        },
    }))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
