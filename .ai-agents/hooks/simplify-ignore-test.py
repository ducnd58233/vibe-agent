from __future__ import annotations

import json
import subprocess
import tempfile
from pathlib import Path


def run_hook(hook_path: Path, payload: dict[str, object], cwd: Path) -> int:
    proc = subprocess.run(
        ["python", str(hook_path)],
        input=json.dumps(payload),
        text=True,
        cwd=str(cwd),
        capture_output=True,
    )
    return proc.returncode


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    hook = repo_root / ".ai-agents" / "hooks" / "simplify-ignore.py"

    with tempfile.TemporaryDirectory() as temp_dir:
        workspace = Path(temp_dir)
        target = workspace / "sample.js"
        target.write_text(
            "const a = 1;\n/* simplify-ignore-start: perf */\nconst x = 42;\n/* simplify-ignore-end */\nconst b = 2;\n",
            encoding="utf-8",
        )

        payload = {"tool_name": "Read", "tool_input": {"file_path": str(target)}}
        read_code = run_hook(hook, payload, workspace)
        if read_code != 0:
            print("FAIL: Read hook failed")
            return 1
        if "BLOCK_" not in target.read_text(encoding="utf-8"):
            print("FAIL: Placeholder was not written")
            return 1
    print("PASS: simplify-ignore Python smoke test")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
