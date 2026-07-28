"""Flag file paths a subagent cited but never opened.

Runs on SubagentStop. Turns the repository's grounding rule ("never describe a file
or path you have not opened; report ACCESS-FAILED instead") from a written
instruction into a sensor.

Reads the Claude Code payload on stdin (`transcript_path`, `stop_hook_active`),
scans the subagent transcript for paths that appear in the final message but in no
tool call or tool result, and reports them via `systemMessage`.

Non-blocking by design: it always exits 0. SubagentStop supports exit 2 to stop a
subagent from finishing, but a heuristic path check produces false positives, and a
wrong block strands real work. Run this warn-only first; switch to blocking only
after the report proves quiet on a real workload.

Transcript layout is probed generically rather than assumed: the scan walks arbitrary
nested JSON and gives up silently if the shape is unfamiliar.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

# Keys whose string values are treated as paths the agent actually touched.
PATH_KEYS = {"file_path", "path", "notebook_path", "file", "filename"}

# A citation must look like a path: a separator plus an extension, or a trailing slash.
PATH_TOKEN = re.compile(r"[\w.@~-]*(?:[\\/][\w.@-]+)+(?:\.[A-Za-z0-9]{1,8})?/?")

# `ACCESS-FAILED: <path>` marks a path the agent correctly reported as unreachable.
ACCESS_FAILED = re.compile(r"ACCESS-FAILED:\s*(\S+)")

MAX_REPORTED = 15


def _read_payload() -> dict[str, object]:
    # Decode explicitly: stdin defaults to the locale encoding (cp1252 on Windows),
    # which corrupts non-ASCII paths before they are compared.
    raw = sys.stdin.buffer.read().decode("utf-8", errors="replace").strip()
    if not raw:
        return {}
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return payload if isinstance(payload, dict) else {}


def _walk(node: object, observed: set[str], texts: list[str]) -> None:
    """Collect touched paths and assistant text from arbitrary nested JSON."""
    if isinstance(node, dict):
        for key, value in node.items():
            if key in PATH_KEYS and isinstance(value, str) and value.strip():
                observed.add(value.strip())
            elif key == "text" and isinstance(value, str):
                texts.append(value)
            else:
                _walk(value, observed, texts)
    elif isinstance(node, list):
        for item in node:
            _walk(item, observed, texts)
    elif isinstance(node, str):
        # Tool results list paths as plain text; treat every path-like token as seen.
        for match in PATH_TOKEN.finditer(node):
            observed.add(match.group(0))


def _normalize(value: str) -> str:
    return value.replace("\\", "/").strip().strip("`'\"()[],.").lstrip("./")


def _load_transcript(path: Path) -> tuple[set[str], list[str]]:
    observed: set[str] = set()
    texts: list[str] = []
    with path.open(encoding="utf-8", errors="replace") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            _walk(entry, observed, texts)
    return observed, texts


def _cited_paths(text: str) -> tuple[list[str], set[str]]:
    """Return path-like tokens in the text, plus those excused by ACCESS-FAILED."""
    cited: list[str] = []
    excused: set[str] = set()

    # A path already reported as unreachable is correctly grounded. Excuse only the
    # path attached to the marker, not everything sharing its line.
    for match in ACCESS_FAILED.finditer(text):
        excused.add(_normalize(match.group(1)))

    for match in PATH_TOKEN.finditer(text):
        token = match.group(0)
        if "." not in token.rsplit("/", 1)[-1] and not token.endswith("/"):
            continue
        cited.append(token)
    return cited, excused


def main() -> int:
    payload = _read_payload()
    if payload.get("stop_hook_active") is True:
        return 0

    transcript_value = payload.get("transcript_path")
    if not isinstance(transcript_value, str) or not transcript_value:
        return 0

    transcript = Path(transcript_value)
    if not transcript.is_file():
        return 0

    try:
        observed, texts = _load_transcript(transcript)
    except OSError:
        return 0

    if not texts:
        return 0

    seen = {_normalize(item) for item in observed}
    seen.discard("")

    cited, excused = _cited_paths(texts[-1])

    unsupported: list[str] = []
    for token in cited:
        normalized = _normalize(token)
        if not normalized or normalized in seen or normalized in excused:
            continue
        if any(normalized in candidate or candidate in normalized for candidate in seen):
            continue
        if normalized not in unsupported:
            unsupported.append(normalized)

    if not unsupported:
        return 0

    shown = unsupported[:MAX_REPORTED]
    extra = len(unsupported) - len(shown)
    detail = "\n".join(f"  - {item}" for item in shown)
    if extra > 0:
        detail += f"\n  - ... and {extra} more"

    print(json.dumps({
        "systemMessage": (
            "subagent-grounding-guard: the final message cites path(s) that do not "
            "appear in any tool call or tool result in this subagent's transcript:\n"
            f"{detail}\n"
            "Verify each one was actually opened. If a path could not be read, report "
            "it as `ACCESS-FAILED: <path>` instead of describing its contents. Paths "
            "being proposed rather than inspected are expected here and can be ignored."
        ),
    }))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
