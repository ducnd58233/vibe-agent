from __future__ import annotations

import json
from pathlib import Path


def main() -> int:
    script_dir = Path(__file__).resolve().parent
    meta_skill = script_dir.parent / "skills" / "using-agent-skills" / "SKILL.md"

    if meta_skill.exists():
        message = (
            "vibe-agent loaded. Use the skill routing flow to choose the right workflow.\n\n"
            + meta_skill.read_text(encoding="utf-8")
        )
        payload = {"priority": "IMPORTANT", "message": message}
    else:
        payload = {
            "priority": "INFO",
            "message": (
                "vibe-agent: using-agent-skills meta-skill not found. "
                "Skills may still be available individually."
            ),
        }

    print(json.dumps(payload))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
