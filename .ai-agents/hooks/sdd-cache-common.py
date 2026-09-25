from __future__ import annotations

import hashlib
import json
import os
import sqlite3
import sys
from pathlib import Path

# Shared by sdd-cache-pre.py and sdd-cache-post.py, loaded via
# importlib.util.spec_from_file_location since a hyphenated filename cannot be
# imported normally. Pulled out after the schema, the path resolution, and the
# stdin decoding were verbatim-duplicated between the two scripts: whichever
# hook fired first on a fresh workspace created the table, and nothing
# enforced that a later edit to one copy of the schema also touched the other.
# A silent column mismatch there would have reproduced, one layer up, the
# exact bug this migration exists to kill (a hook that fails quietly and never
# reports it).


def read_input() -> dict[str, object]:
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


def project_root() -> Path:
    return Path(os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd()))


def memory_db_path() -> Path:
    # The runtime passes the resolved path so both sides cannot drift.
    # The fallback is for a standalone run, and names the same layout.
    configured = os.environ.get("VIBE_MEMORY_DB_PATH", "").strip()
    if configured:
        return Path(configured)
    return project_root() / ".agent-state" / "memory.db"


SCHEMA = """
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


def open_cache_db() -> sqlite3.Connection:
    path = memory_db_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    # busy_timeout is per-connection, not stored in the file, so it is set here
    # too even though the Go side already sets it on its own connections.
    conn = sqlite3.connect(str(path), timeout=5)
    conn.execute("PRAGMA busy_timeout = 5000")
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute(SCHEMA)
    return conn


def cache_key_for_url(url: str) -> str:
    return hashlib.sha256(url.encode("utf-8")).hexdigest()[:32]
