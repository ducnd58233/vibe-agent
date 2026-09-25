#!/usr/bin/env python3
"""Offline round-trip check for the sdd-cache hooks' SQLite table.

Both hooks make a network call before touching the cache (a HEAD request),
so a test that exercises main() needs the network. This instead drives the
shared sdd-cache-common.py module directly: post's exact INSERT shape, pre's
exact SELECT shape, against a temp copy of the one schema both hooks load
from that single file - so there is nothing left to drift between them.

Also asserts pre.py and post.py both actually load sdd-cache-common.py rather
than keeping their own copy of the schema, since a second copy is exactly how
this table's predecessor bug (silent, unreported divergence) would return.

Picked up automatically by scripts/pre-commit.sh's `.ai-agents/hooks/*-test.py`
loop when a staged change touches this directory.

Usage: python3 .ai-agents/hooks/sdd-cache-test.py
"""

from __future__ import annotations

import importlib.util
import os
import sqlite3
import sys
import tempfile
import time
from pathlib import Path
from types import ModuleType


def _load(path: Path, name: str) -> ModuleType:
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main() -> int:
    hooks = Path(__file__).resolve().parent
    common = _load(hooks / "sdd-cache-common.py", "sdd_cache_common")

    failures = 0

    for name in ("sdd-cache-pre.py", "sdd-cache-post.py"):
        source = (hooks / name).read_text(encoding="utf-8")
        if "sdd-cache-common.py" not in source:
            failures += 1
            print(f"  FAIL  {name} does not load sdd-cache-common.py", file=sys.stderr)
        elif "CREATE TABLE" in source:
            failures += 1
            print(f"  FAIL  {name} still declares its own schema instead of loading sdd-cache-common.py", file=sys.stderr)
        else:
            print(f"  ok    {name} loads the shared module and has no schema of its own")

    with tempfile.TemporaryDirectory() as tmp:
        db_path = Path(tmp) / "memory.db"
        os.environ["VIBE_MEMORY_DB_PATH"] = str(db_path)

        conn = common.open_cache_db()
        now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        url = "https://example.com/contract-check"
        key = common.cache_key_for_url(url)
        conn.execute(
            """
            INSERT INTO sdd_cache
                (key, url, prompt, etag, last_modified, content, fetched_at,
                 created_by, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?)
            """,
            (key, url, "the prompt", 'W/"abc"', "Mon, 01 Jan 2026 00:00:00 GMT",
             "cached body", int(time.time()), now, now),
        )
        conn.commit()

        rows = conn.execute("PRAGMA table_info(sdd_cache)").fetchall()
        present = {row[1] for row in rows}
        wanted = {
            "key", "url", "prompt", "etag", "last_modified", "content",
            "fetched_at", "created_by", "reviewed_by_agents", "created_at",
            "updated_at",
        }
        missing = wanted - present
        if missing:
            failures += 1
            print(f"  FAIL  sdd_cache is missing columns: {sorted(missing)}", file=sys.stderr)
        else:
            print("  ok    sdd_cache has every column both hooks and the provenance contract expect")

        row = conn.execute(
            "SELECT prompt, etag, last_modified, content, fetched_at "
            "FROM sdd_cache WHERE key = ?",
            (key,),
        ).fetchone()
        conn.close()

        if row is None:
            failures += 1
            print("  FAIL  the row just written could not be read back", file=sys.stderr)
        elif row[0] != "the prompt" or row[3] != "cached body":
            failures += 1
            print(f"  FAIL  round-tripped row does not match what was written: {row}", file=sys.stderr)
        else:
            print("  ok    a row written through the shared open_cache_db is read back correctly")

    # Both hooks catch sqlite3.Error around every write/read and fail silent
    # (return 0) rather than crash a tool call over cache bookkeeping. This
    # confirms that assumption against the real exception type SQLite raises
    # once a lock is held past a connection's own busy_timeout, not just
    # against a short, uncontended hold like the Go/Python concurrency test
    # above (which only proves the wait succeeds within the timeout).
    with tempfile.TemporaryDirectory() as tmp:
        db_path = Path(tmp) / "locked.db"
        holder = sqlite3.connect(str(db_path), timeout=5)
        holder.execute("PRAGMA journal_mode = WAL")
        holder.execute("CREATE TABLE t (id INTEGER PRIMARY KEY)")
        holder.execute("BEGIN IMMEDIATE")
        holder.execute("INSERT INTO t DEFAULT VALUES")

        waiter = sqlite3.connect(str(db_path), timeout=0.2)
        try:
            waiter.execute("INSERT INTO t DEFAULT VALUES")
        except sqlite3.Error:
            print("  ok    a write past busy_timeout raises sqlite3.Error, matching both hooks' except clause")
        else:
            failures += 1
            print("  FAIL  a write past busy_timeout did not raise sqlite3.Error", file=sys.stderr)
        finally:
            waiter.close()
            holder.rollback()
            holder.close()

    if failures:
        print(f"\nsdd-cache contract check: {failures} failure(s)", file=sys.stderr)
        return 1
    print("\nsdd-cache contract check: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
