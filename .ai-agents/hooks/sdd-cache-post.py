from __future__ import annotations

import hashlib
import json
import os
import sqlite3
import sys
import time
import urllib.request
from pathlib import Path


def _read_input() -> dict[str, object]:
    # Decode explicitly: stdin defaults to the locale encoding (cp1252 on Windows),
    # which corrupts non-ASCII URLs and paths before they are resolved.
    raw = sys.stdin.buffer.read().decode("utf-8", errors="replace").strip()
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


def _memory_db_path() -> Path:
    # The runtime passes the resolved path so both sides cannot drift.
    # The fallback is for a standalone run, and names the same layout.
    configured = os.environ.get("VIBE_MEMORY_DB_PATH", "").strip()
    if configured:
        return Path(configured)
    return _project_root() / ".agent-state" / "memory.db"


_SCHEMA = """
CREATE TABLE IF NOT EXISTS sdd_cache (
    key                TEXT PRIMARY KEY,
    url                TEXT NOT NULL,
    prompt             TEXT NOT NULL DEFAULT '',
    etag               TEXT NOT NULL DEFAULT '',
    last_modified      TEXT NOT NULL DEFAULT '',
    content            TEXT NOT NULL,
    fetched_at         INTEGER NOT NULL,
    created_by         TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
)
"""


def _open_cache_db() -> sqlite3.Connection:
    path = _memory_db_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(str(path), timeout=5)
    conn.execute("PRAGMA busy_timeout = 5000")
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute(_SCHEMA)
    return conn


def _cache_key_for_url(url: str) -> str:
    return hashlib.sha256(url.encode("utf-8")).hexdigest()[:32]


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

    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    try:
        conn = _open_cache_db()
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
                _cache_key_for_url(url),
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
