#!/usr/bin/env python3
"""Offline round-trip check for the sdd-cache hooks' SQLite table.

Both hooks make a network call before touching the cache (a HEAD request),
so a test that exercises main() needs the network. This instead loads each
script as a module and drives its database functions directly: post's exact
INSERT, pre's exact SELECT, against a temp copy of the schema. This is the
contract test across the language boundary Go and Python share memory.db
through.

Picked up automatically by scripts/pre-commit.sh's `.ai-agents/hooks/*-test.py`
loop when a staged change touches this directory.

Usage: python3 .ai-agents/hooks/sdd-cache-test.py
"""

from __future__ import annotations

import importlib.util
import os
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
    pre = _load(hooks / "sdd-cache-pre.py", "sdd_cache_pre")
    post = _load(hooks / "sdd-cache-post.py", "sdd_cache_post")

    failures = 0
    with tempfile.TemporaryDirectory() as tmp:
        db_path = Path(tmp) / "memory.db"
        os.environ["VIBE_MEMORY_DB_PATH"] = str(db_path)

        # post's own _open_cache_db, so this exercises its real schema DDL,
        # not a copy of it written for this check.
        conn = post._open_cache_db()
        now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        url = "https://example.com/contract-check"
        key = post._cache_key_for_url(url)
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
        conn.close()

        # pre reads through its own _open_cache_db against the same file, and
        # must find the row post just wrote with pre's own SELECT shape.
        conn = pre._open_cache_db()
        row = conn.execute(
            "SELECT prompt, etag, last_modified, content, fetched_at "
            "FROM sdd_cache WHERE key = ?",
            (pre._cache_key_for_url(url),),
        ).fetchone()
        conn.close()

        if row is None:
            failures += 1
            print("  FAIL  pre-cache could not find the row post-cache wrote", file=sys.stderr)
        elif row[0] != "the prompt" or row[3] != "cached body":
            failures += 1
            print(f"  FAIL  round-tripped row does not match what was written: {row}", file=sys.stderr)
        else:
            print("  ok    a row written by sdd-cache-post is read back correctly by sdd-cache-pre")

        if pre._cache_key_for_url(url) != post._cache_key_for_url(url):
            failures += 1
            print("  FAIL  pre and post derive different keys for the same url", file=sys.stderr)
        else:
            print("  ok    pre and post derive the same cache key for the same url")

    if failures:
        print(f"\nsdd-cache contract check: {failures} failure(s)", file=sys.stderr)
        return 1
    print("\nsdd-cache contract check: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
