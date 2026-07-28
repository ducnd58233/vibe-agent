from __future__ import annotations

import hashlib
import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import cast


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


def _cache_file_for_url(url: str) -> Path:
    key = hashlib.sha256(url.encode("utf-8")).hexdigest()[:32]
    return _project_root() / ".claude" / "sdd-cache" / f"{key}.json"


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

    cache_file = _cache_file_for_url(url)
    if not cache_file.exists():
        return 0

    try:
        cache_raw = json.loads(cache_file.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return 0
    if not isinstance(cache_raw, dict):
        return 0

    etag_value = cache_raw.get("etag")
    last_modified_value = cache_raw.get("last_modified")
    etag = etag_value if isinstance(etag_value, str) else ""
    last_modified = last_modified_value if isinstance(last_modified_value, str) else ""
    if not etag and not last_modified:
        return 0

    status = _http_head(url, etag, last_modified)
    if status != 304:
        return 0

    content_value = cache_raw.get("content")
    if not isinstance(content_value, str) or not content_value:
        return 0

    fetched_at_value = cache_raw.get("fetched_at")
    fetched_at = int(fetched_at_value) if isinstance(fetched_at_value, int) else 0
    timestamp = (
        time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(fetched_at))
        if fetched_at > 0
        else "unknown"
    )
    prompt_value = cache_raw.get("prompt")
    original_prompt = prompt_value if isinstance(prompt_value, str) else ""

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
