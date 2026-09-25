from __future__ import annotations

import importlib.util
import sqlite3
import sys
import time
import urllib.error
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
    payload = _common.read_input()
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict):
        return 0

    url = tool_input.get("url")
    if not isinstance(url, str) or not url:
        return 0

    try:
        conn = _common.open_cache_db()
    except Exception:
        return 0
    try:
        row = conn.execute(
            "SELECT prompt, etag, last_modified, content, fetched_at "
            "FROM sdd_cache WHERE key = ?",
            (_common.cache_key_for_url(url),),
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
