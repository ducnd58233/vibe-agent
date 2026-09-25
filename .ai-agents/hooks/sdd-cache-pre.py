from __future__ import annotations

import hashlib
import json
import os
import sqlite3
import sys
import time
import urllib.error
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
    # busy_timeout is per-connection, not stored in the file, so it is set here
    # too even though the Go side already sets it on its own connections.
    conn = sqlite3.connect(str(path), timeout=5)
    conn.execute("PRAGMA busy_timeout = 5000")
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute(_SCHEMA)
    return conn


def _cache_key_for_url(url: str) -> str:
    return hashlib.sha256(url.encode("utf-8")).hexdigest()[:32]


def _http_head(url: str, etag: str, last_modified: str) -> int:
    request = urllib.request.Request(url=url, method="HEAD")
    if etag:
        request.add_header("If-None-Match", etag)
    if last_modified:
        request.add_header("If-Modified-Since", last_modified)
    try:
        with urllib.request.urlopen(request, timeout=5) as response:
            return response.status
    except urllib.error.HTTPError as exc:
        return exc.code
    except Exception:
        return 0


def main() -> int:
    payload = _read_input()
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict):
        return 0

    url = tool_input.get("url")
    if not isinstance(url, str) or not url:
        return 0

    try:
        conn = _open_cache_db()
    except Exception:
        return 0
    try:
        row = conn.execute(
            "SELECT prompt, etag, last_modified, content, fetched_at "
            "FROM sdd_cache WHERE key = ?",
            (_cache_key_for_url(url),),
        ).fetchone()
    except sqlite3.Error:
        return 0
    finally:
        conn.close()

    if row is None:
        return 0
    original_prompt, etag, last_modified, content_value, fetched_at_raw = row
    if not etag and not last_modified:
        return 0
    if not content_value:
        return 0

    status = _http_head(url, etag, last_modified)
    if status != 304:
        return 0

    fetched_at = fetched_at_raw if isinstance(fetched_at_raw, int) else 0
    timestamp = (
        time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(fetched_at))
        if fetched_at > 0
        else "unknown"
    )

    print(f"[sdd-cache] Cache hit for {url}", file=sys.stderr)
    print("", file=sys.stderr)
    print(
        f"Revalidated via HTTP 304; unchanged since {timestamp}.",
        file=sys.stderr,
    )
    print("", file=sys.stderr)
    if original_prompt:
        print(f'Original WebFetch prompt: "{original_prompt}"', file=sys.stderr)
        print("", file=sys.stderr)
    print("----- BEGIN CACHED CONTENT -----", file=sys.stderr)
    print(content_value, file=sys.stderr)
    print("----- END CACHED CONTENT -----", file=sys.stderr)

    return 2


if __name__ == "__main__":
    raise SystemExit(main())
