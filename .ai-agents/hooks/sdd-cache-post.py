from __future__ import annotations

import hashlib
import json
import os
import sys
import time
import urllib.request
from pathlib import Path


def _read_input() -> dict[str, object]:
    raw = sys.stdin.read().strip()
    if not raw:
        return {}
    try:
        loaded = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return loaded if isinstance(loaded, dict) else {}


def _project_root() -> Path:
    return Path(os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd()))


def _extract_content(tool_response: object) -> str:
    if isinstance(tool_response, str):
        return tool_response
    if not isinstance(tool_response, dict):
        return ""
    for field in ("result", "output", "text", "content", "body"):
        value = tool_response.get(field)
        if isinstance(value, str) and value:
            return value
    return ""


def _head_validators(url: str) -> tuple[str, str]:
    request = urllib.request.Request(url=url, method="HEAD")
    with urllib.request.urlopen(request, timeout=5) as response:
        etag = response.headers.get("ETag", "")
        last_modified = response.headers.get("Last-Modified", "")
    return etag, last_modified


def main() -> int:
    payload = _read_input()
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict):
        return 0

    url = tool_input.get("url")
    prompt = tool_input.get("prompt")
    if not isinstance(url, str) or not url:
        return 0
    prompt_text = prompt if isinstance(prompt, str) else ""

    content = _extract_content(payload.get("tool_response"))
    if not content:
        return 0

    try:
        etag, last_modified = _head_validators(url)
    except Exception:
        return 0
    if not etag and not last_modified:
        return 0

    cache_dir = _project_root() / ".claude" / "sdd-cache"
    cache_dir.mkdir(parents=True, exist_ok=True)
    key = hashlib.sha256(url.encode("utf-8")).hexdigest()[:32]
    cache_file = cache_dir / f"{key}.json"

    record = {
        "url": url,
        "prompt": prompt_text,
        "etag": etag,
        "last_modified": last_modified,
        "content": content,
        "fetched_at": int(time.time()),
    }
    cache_file.write_text(json.dumps(record, ensure_ascii=False), encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
