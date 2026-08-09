#!/usr/bin/env python3
"""Validate the YAML frontmatter of every asset under .ai-agents/.

Invalid frontmatter fails silently, which is why this check exists. A loader that
cannot parse the block does not error: it falls back to the first line of the body
and shows that as the asset's description. `commands/harden.md` carried

    description: Harden AI assets: permissions, hooks, tool boundaries

for months. The unquoted second colon makes it invalid YAML, so the command was
advertised by whatever prose happened to sit at the top of the file. Nothing in the
repo noticed, because every other check reads the file as Markdown.

Checks:

  1. The frontmatter block parses as YAML and is a mapping.
  2. It carries a description, since that is what the routers and the harness show.
  3. The description is not empty and not a stray XML section tag, which is the
     shape the fallback produces once the body is tagged.

Needs PyYAML (scripts/requirements.txt).

Usage: python3 scripts/check-frontmatter.py [repo-root]
"""

from __future__ import annotations

import pathlib
import re
import sys

import yaml

FRONTMATTER = re.compile(r"^---\n(.*?)\n---\n", re.S)
SECTION_TAG = re.compile(r"^<[a-z_]+>$")

# These carry no frontmatter by design: routers, indexes, and authoring templates
# are read by people and by agents as prose, not loaded as addressable assets.
OPTIONAL = ("ROUTER.md", "README.md", "TEMPLATE.md", "MEMORY.md")


def main(root: pathlib.Path) -> int:
    assets = root / ".ai-agents"
    if not assets.is_dir():
        print(f"check-frontmatter: no {assets}", file=sys.stderr)
        return 1

    problems: list[str] = []
    checked = 0

    for path in sorted(assets.rglob("*.md")):
        rel = path.relative_to(root).as_posix()
        text = path.read_text(encoding="utf-8")
        match = FRONTMATTER.match(text)

        if not match:
            if path.name.endswith(OPTIONAL):
                continue
            # A skill or agent without frontmatter cannot be selected by name.
            if path.name == "SKILL.md" or "/agents/" in rel or "/commands/" in rel:
                problems.append(f"{rel}: no YAML frontmatter block")
            continue

        checked += 1
        try:
            data = yaml.safe_load(match.group(1))
        except yaml.YAMLError as exc:
            first = str(exc).split("\n")[0]
            problems.append(f"{rel}: frontmatter is not valid YAML ({first})")
            problems.append(f"{rel}: a value holding a colon must be quoted")
            continue

        if not isinstance(data, dict):
            problems.append(f"{rel}: frontmatter parses but is not a mapping")
            continue

        description = data.get("description")
        if description is None:
            problems.append(f"{rel}: frontmatter has no description")
            continue

        flat = " ".join(str(description).split())
        if not flat:
            problems.append(f"{rel}: description is empty")
        elif SECTION_TAG.match(flat):
            problems.append(f"{rel}: description is the section tag {flat}; "
                            "the frontmatter is not being read")

    for line in problems:
        print(line)

    if problems:
        print()
        print(f"{len(problems)} frontmatter problem(s) across {checked} assets.")
        return 1

    print(f"check-frontmatter: OK ({checked} assets)")
    return 0


if __name__ == "__main__":
    where = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else pathlib.Path(__file__).resolve().parent.parent
    raise SystemExit(main(where))
