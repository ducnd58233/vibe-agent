from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import sys
from pathlib import Path

START_PATTERN = re.compile(r"simplify-ignore-start(?:\s*:\s*(.*?))?\s*(?:\*/|-->)?\s*$")
END_PATTERN = re.compile(r"simplify-ignore-end")
BLOCK_PATTERN = re.compile(r"BLOCK_([0-9a-f]{8})(?::\s*(.*?))?$")


def _project_root() -> Path:
    return Path(os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd()))


def _cache_root() -> Path:
    return _project_root() / ".claude" / ".simplify-ignore-cache"


def _read_payload() -> dict[str, object]:
    raw = sys.stdin.read().strip()
    if not raw:
        return {}
    try:
        loaded = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return loaded if isinstance(loaded, dict) else {}


def _file_id(path: Path) -> str:
    return hashlib.sha1(str(path).encode("utf-8")).hexdigest()[:16]


def _block_id(content: str) -> str:
    return hashlib.sha1(content.encode("utf-8")).hexdigest()[:8]


def _state_files(cache: Path, fid: str) -> tuple[Path, Path]:
    return cache / f"{fid}.bak", cache / f"{fid}.path"


def _restore_all() -> int:
    cache = _cache_root()
    if not cache.exists():
        return 0
    for bak in cache.glob("*.bak"):
        fid = bak.stem
        path_ref = cache / f"{fid}.path"
        if not path_ref.exists():
            continue
        target = Path(path_ref.read_text(encoding="utf-8"))
        if target.exists():
            shutil.copyfile(bak, target)
        bak.unlink(missing_ok=True)
        path_ref.unlink(missing_ok=True)
        for sidecar in cache.glob(f"{fid}.block.*.txt"):
            sidecar.unlink(missing_ok=True)
    return 0


def _parse_blocks(content: str) -> tuple[str, dict[str, str], bool]:
    lines = content.splitlines()
    output_lines: list[str] = []
    blocks: dict[str, str] = {}
    in_block = False
    block_lines: list[str] = []
    reason = ""
    found = False

    for line in lines:
        if not in_block:
            start_match = START_PATTERN.search(line)
            if start_match:
                in_block = True
                found = True
                reason = (start_match.group(1) or "").strip()
                block_lines = [line]
                if END_PATTERN.search(line):
                    in_block = False
                    block_text = "\n".join(block_lines)
                    bid = _block_id(block_text)
                    blocks[bid] = block_text
                    placeholder = f"BLOCK_{bid}"
                    if reason:
                        placeholder += f": {reason}"
                    output_lines.append(placeholder)
                continue
            output_lines.append(line)
            continue

        block_lines.append(line)
        if END_PATTERN.search(line):
            in_block = False
            block_text = "\n".join(block_lines)
            bid = _block_id(block_text)
            blocks[bid] = block_text
            placeholder = f"BLOCK_{bid}"
            if reason:
                placeholder += f": {reason}"
            output_lines.append(placeholder)

    if in_block:
        output_lines.extend(block_lines)
    return "\n".join(output_lines), blocks, found


def _save_blocks(cache: Path, fid: str, blocks: dict[str, str]) -> None:
    for path in cache.glob(f"{fid}.block.*.txt"):
        path.unlink(missing_ok=True)
    for bid, block in blocks.items():
        (cache / f"{fid}.block.{bid}.txt").write_text(block, encoding="utf-8")


def _load_blocks(cache: Path, fid: str) -> dict[str, str]:
    blocks: dict[str, str] = {}
    for path in cache.glob(f"{fid}.block.*.txt"):
        parts = path.name.split(".")
        if len(parts) >= 4:
            bid = parts[2]
            blocks[bid] = path.read_text(encoding="utf-8")
    return blocks


def _expand_placeholders(content: str, blocks: dict[str, str]) -> str:
    output_lines: list[str] = []

    for line in content.splitlines():
        def _replace_match(match: re.Match[str]) -> str:
            bid = match.group(1)
            block = blocks.get(bid)
            return block if block is not None else match.group(0)

        replaced = BLOCK_PATTERN.sub(_replace_match, line)
        output_lines.append(replaced)

    return "\n".join(output_lines)


def _handle_read(file_path: Path) -> int:
    if not file_path.exists():
        return 0
    text = file_path.read_text(encoding="utf-8")
    filtered, blocks, found = _parse_blocks(text)
    if not found:
        return 0

    cache = _cache_root()
    cache.mkdir(parents=True, exist_ok=True)
    fid = _file_id(file_path)
    bak, path_ref = _state_files(cache, fid)
    if bak.exists():
        return 0

    bak.write_text(text, encoding="utf-8")
    path_ref.write_text(str(file_path), encoding="utf-8")
    _save_blocks(cache, fid, blocks)
    file_path.write_text(filtered, encoding="utf-8")
    return 0


def _handle_write_like(file_path: Path) -> int:
    cache = _cache_root()
    fid = _file_id(file_path)
    bak, _ = _state_files(cache, fid)
    if not bak.exists() or not file_path.exists():
        return 0

    blocks = _load_blocks(cache, fid)
    if not blocks:
        return 0

    current = file_path.read_text(encoding="utf-8")
    expanded = _expand_placeholders(current, blocks)
    bak.write_text(expanded, encoding="utf-8")
    filtered, refreshed_blocks, _ = _parse_blocks(expanded)
    _save_blocks(cache, fid, refreshed_blocks)
    file_path.write_text(filtered, encoding="utf-8")
    return 0


def main() -> int:
    payload = _read_payload()
    tool_name_value = payload.get("tool_name")
    if not isinstance(tool_name_value, str) or not tool_name_value:
        return _restore_all()

    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict):
        return 0
    file_path_value = tool_input.get("file_path")
    if not isinstance(file_path_value, str) or not file_path_value:
        return 0

    file_path = Path(file_path_value)
    if tool_name_value == "Read":
        return _handle_read(file_path)
    if tool_name_value in {"Edit", "Write"}:
        return _handle_write_like(file_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())