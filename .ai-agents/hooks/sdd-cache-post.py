from __future__ import annotations

import importlib.util
import sqlite3
import time
import urllib.request
from pathlib import Path


def _load_common():
    spec = importlib.util.spec_from_file_location(
        "sdd_cache_common", Path(__file__).resolve().parent / "sdd-cache-common.py"
    )
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load sdd-cache-common.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


_common = _load_common()


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
    payload = _common.read_input()
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

    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    try:
        conn = _common.open_cache_db()
    except Exception:
        return 0
    try:
        conn.execute(
            """
            INSERT INTO sdd_cache
                (key, url, prompt, etag, last_modified, content, fetched_at,
                 created_by, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?)
            ON CONFLICT(key) DO UPDATE SET
                prompt        = excluded.prompt,
                etag          = excluded.etag,
                last_modified = excluded.last_modified,
                content       = excluded.content,
                fetched_at    = excluded.fetched_at,
                updated_at    = excluded.updated_at
            """,
            (
                _common.cache_key_for_url(url),
                url,
                prompt_text,
                etag,
                last_modified,
                content,
                int(time.time()),
                now,
                now,
            ),
        )
        conn.commit()
    except sqlite3.Error:
        return 0
    finally:
        conn.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
