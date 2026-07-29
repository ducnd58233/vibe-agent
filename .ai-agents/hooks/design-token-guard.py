from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path


HEX_COLOR = re.compile(r"(?<![A-Za-z0-9_-])#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3})?(?:[0-9a-fA-F]{2})?(?![A-Za-z0-9_-])")
TARGET_SUFFIXES = {".css", ".scss", ".sass", ".less", ".tsx", ".jsx", ".ts", ".js", ".vue", ".svelte"}


def _read_input() -> dict[str, object]:
    # Decode explicitly: stdin defaults to the locale encoding (cp1252 on Windows),
    # which corrupts non-ASCII file paths before they are resolved.
    raw = sys.stdin.buffer.read().decode("utf-8", errors="replace").strip()
    if not raw:
        return {}
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return data if isinstance(data, dict) else {}


def _candidate_path(payload: dict[str, object]) -> Path | None:
    # Claude PostToolUse nests the path under tool_input; Cursor afterFileEdit
    # puts it at the top level. Accept both so one script serves both harnesses.
    tool_input = payload.get("tool_input")
    if isinstance(tool_input, dict):
        for key in ("file_path", "path"):
            value = tool_input.get(key)
            if isinstance(value, str) and value:
                return Path(value)
    for key in ("file_path", "path"):
        value = payload.get(key)
        if isinstance(value, str) and value:
            return Path(value)
    return None


def main() -> int:
    payload = _read_input()
    path = _candidate_path(payload)
    if path is None or path.suffix.lower() not in TARGET_SUFFIXES:
        return 0

    project_root = Path(os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd()))
    full_path = path if path.is_absolute() else project_root / path
    if not full_path.exists() or not full_path.is_file():
        return 0

    try:
        text = full_path.read_text(encoding="utf-8")
    except OSError:
        return 0

    if "design-token-guard: allow-raw-color" in text:
        return 0

    matches = sorted(set(HEX_COLOR.findall(text)))
    if not matches:
        return 0

    sample = ", ".join(matches[:8])
    message = (
        f"[design-token-guard] {path} contains raw color values ({sample}). "
        "Prefer semantic design tokens/CSS variables unless this is an intentional exception. "
        "Add 'design-token-guard: allow-raw-color' near the file header to acknowledge."
    )
    print(message, file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
