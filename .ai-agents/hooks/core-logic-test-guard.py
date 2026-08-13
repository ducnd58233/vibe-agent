"""Warn on tests that cannot fail when product behavior changes.

The core-logic rule in the test-driven-development skill is prose, which means
the model decides whether to follow it. This is the deterministic half: the host
fires it on every write to a test file, and it reports what it can prove from
the text.

Binary pattern checks only, no model calls. Always exits 0, so it warns without
blocking the edit.

WHAT THIS DELIBERATELY DOES NOT CHECK

The skill's bans are semantic, and most of them cannot be decided from syntax.
Reading a file inside a test is the clearest example: "the config file exists" is
discovery, while "saving writes a manifest" and "this input creates no file" are
behavior, and all three call the same function. A guard that flagged the API
would be wrong about two cases out of three, so the checks below fire only where
the text alone settles it:

  - a test with no assertion cannot fail for any reason
  - a test whose only assertion is an environment variable tests the machine
  - a test whose only assertion is that a mock was called tests the wiring
  - a name promising discovery, plus a matching lone assertion, is discovery

Everything else stays with the reviewer and the skill. Opt out on a deliberate
case with a `core-logic-test-guard: allow` comment on or above the test.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ALLOW_MARKER = "core-logic-test-guard: allow"

# --- what counts as a test file ----------------------------------------------

GO_SUFFIX = "_test.go"
PY_PREFIX = "test_"
PY_SUFFIX = "_test.py"
JS_SUFFIXES = {".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
JS_MARKERS = (".test.", ".spec.")


# --- per-language shapes -----------------------------------------------------
#
# Each language contributes a way to find test blocks and a way to recognise an
# assertion inside one. Block boundaries are found by the next declaration at the
# same level, which over-approximates a block's end rather than under: a block
# that swallows a trailing helper reports fewer findings, never more.

GO_TEST = re.compile(r"^func\s+(Test\w+)\s*\(", re.MULTILINE)
GO_BLOCK_END = re.compile(r"^func\s", re.MULTILINE)
GO_ASSERT = re.compile(r"\bt\.(Error|Errorf|Fatal|Fatalf)\b|\b(?:require|assert)\.\w+\(")
GO_EXEMPT = re.compile(r"\bt\.Skip(f|Now)?\b")

JS_TEST = re.compile(
    r"""^[ \t]*(?:it|test)(?:\.\w+)?\s*\(\s*['"`](?P<name>[^'"`]+)['"`]""",
    re.MULTILINE,
)
JS_ASSERT = re.compile(r"\bexpect\s*\(|\bassert\b|\.should\b")
JS_EXEMPT = re.compile(r"^[ \t]*(?:it|test)\.(?:skip|todo)\b", re.MULTILINE)

PY_TEST = re.compile(r"^(?P<indent>[ \t]*)def\s+(?P<name>test_\w+)\s*\(", re.MULTILINE)
PY_BLOCK_END = re.compile(r"^[ \t]*(?:def|class)\s", re.MULTILINE)
PY_ASSERT = re.compile(r"^\s*assert\b|\bself\.assert\w*\(|\bpytest\.raises\b", re.MULTILINE)
PY_EXEMPT = re.compile(r"@pytest\.mark\.skip|\bpytest\.skip\(")


# --- the four things the text can settle -------------------------------------

ENV_ONLY = re.compile(
    r"\bos\.Getenv\s*\(|\bos\.environ\b|\bprocess\.env\b|\bSystem\.getenv\s*\("
)

# A lone "the mock was called" is the pass-through wrapper smell: delete the
# wrapper body and the dependency is still reachable, so nothing fails.
MOCK_ONLY = re.compile(
    r"toHaveBeenCalled(?:Times|With)?\s*\(|\.assert_called(?:_once)?(?:_with)?\s*\(|"
    r"\bAssertExpectations\s*\(|\bverify\s*\("
)

EXISTENCE = re.compile(
    r"\bos\.Stat\s*\(|\bexistsSync\s*\(|\bos\.path\.exists\s*\(|"
    r"\.exists\s*\(\s*\)|\.is_file\s*\(\s*\)|\.is_dir\s*\(\s*\)|\bfs\.access\s*\("
)

# Names that promise the test is about the environment rather than the product.
DISCOVERY_NAME = re.compile(
    r"exists|is[_ ]?present|has[_ ]?file|folder|directory|layout|structure|scaffold",
    re.IGNORECASE,
)
HEALTH_NAME = re.compile(
    r"is[_ ]?up|is[_ ]?running|is[_ ]?alive|started|container|health|ping|"
    r"connects?|connection|reachable",
    re.IGNORECASE,
)


# --- what is code, and what is only quoted ------------------------------------
#
# A test for a parser, a linter, or a code generator states its input as source
# code, and that source starts lines with `func`, `def`, or `class` in column
# one. Read literally it breaks the guard in both directions:
#
#   - a block ends at the first declaration inside the fixture, hiding every
#     assertion that follows it. That is a false positive on precisely the tests
#     that most need checking.
#   - an assertion quoted inside a fixture counts as a real one, so a test that
#     checks nothing passes because its own example says `assert`.
#
# Both go away by blanking the inside of literals and comments before anything
# else looks at the text. Length and newlines are preserved, so every offset and
# reported line number stays the same as in the file the human will open.
#
# Openers are tried longest first, so Python's triple quote wins over its single
# one. `escapes` marks the literals where a backslash defers the closer; Go's
# raw string and Python's triple quote take a backslash as an ordinary byte.
LITERALS = {
    "go": {
        "comments": (("//", "\n"), ("/*", "*/")),
        "strings": (("`", "`", False), ('"', '"', True), ("'", "'", True)),
    },
    "python": {
        "comments": (("#", "\n"),),
        "strings": (('"""', '"""', False), ("'''", "'''", False),
                    ('"', '"', True), ("'", "'", True)),
    },
    "js": {
        "comments": (("//", "\n"), ("/*", "*/")),
        "strings": (("`", "`", True), ('"', '"', True), ("'", "'", True)),
    },
}


def _blank(span: str) -> str:
    """Replace a span with spaces, keeping its newlines so lines still count."""
    return "".join("\n" if char == "\n" else " " for char in span)


def _mask_literals(language: str, text: str) -> str:
    """Return text with the inside of every literal and comment blanked out."""
    spec = LITERALS[language]
    comments = sorted(spec["comments"], key=lambda pair: -len(pair[0]))
    strings = sorted(spec["strings"], key=lambda triple: -len(triple[0]))

    out: list[str] = []
    index, size = 0, len(text)
    while index < size:
        for opener, closer in comments:
            if text.startswith(opener, index):
                end = text.find(closer, index + len(opener))
                # An unterminated comment runs to the end of the file. A line
                # comment keeps its newline, which is the closer itself.
                stop = size if end < 0 else end + (0 if closer == "\n" else len(closer))
                out.append(opener + _blank(text[index + len(opener):stop]))
                index = stop
                break
        else:
            for opener, closer, escapes in strings:
                if not text.startswith(opener, index):
                    continue
                cursor = index + len(opener)
                while cursor < size:
                    if escapes and text[cursor] == "\\":
                        cursor += 2
                        continue
                    if text.startswith(closer, cursor):
                        break
                    cursor += 1
                stop = min(cursor + len(closer), size)
                out.append(opener + _blank(text[index + len(opener):cursor]) +
                           text[cursor:stop])
                index = stop
                break
            else:
                out.append(text[index])
                index += 1
    return "".join(out)


class Block:
    """One test and the text of its body.

    Two copies of the body, because the two questions want different text.
    `text` is masked and answers "does this check anything", where a literal is
    data. `source` is verbatim and answers "did someone opt out", where the
    marker lives in a comment that masking blanks.
    """

    def __init__(self, name: str, text: str, source: str, line: int) -> None:
        self.name = name
        self.text = text
        self.source = source
        self.line = line


def _read_payload() -> dict[str, object]:
    # Decode explicitly: stdin defaults to the locale encoding (cp1252 on
    # Windows), which corrupts non-ASCII paths before they are resolved.
    raw = sys.stdin.buffer.read().decode("utf-8", errors="replace").strip()
    if not raw:
        return {}
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return {}
    return payload if isinstance(payload, dict) else {}


def _target_file(payload: dict[str, object]) -> Path | None:
    # Claude PostToolUse fires for every tool, so the tool name has to be
    # checked. Cursor afterFileEdit is already edit-only and sends no tool_name,
    # so its absence is not a reason to bail out.
    tool_name = payload.get("tool_name")
    if tool_name is not None and tool_name not in {"Edit", "Write", "NotebookEdit"}:
        return None

    # Claude nests the path under tool_input; Cursor puts it at the top level.
    value: object = None
    tool_input = payload.get("tool_input")
    if isinstance(tool_input, dict):
        value = tool_input.get("file_path")
    if not isinstance(value, str) or not value:
        value = payload.get("file_path")
    if not isinstance(value, str) or not value:
        return None

    path = Path(value)
    return path if _language(path) and path.is_file() else None


def _language(path: Path) -> str | None:
    name = path.name
    if name.endswith(GO_SUFFIX):
        return "go"
    if name.endswith(PY_SUFFIX) or (name.startswith(PY_PREFIX) and path.suffix == ".py"):
        return "python"
    if path.suffix.lower() in JS_SUFFIXES and any(m in name for m in JS_MARKERS):
        return "js"
    return None


def _line_of(text: str, offset: int) -> int:
    return text.count("\n", 0, offset) + 1


def _blocks(language: str, masked: str, source: str) -> list[Block]:
    """Find test blocks in the masked text, carrying the verbatim body along.

    Boundaries come from the masked copy so a declaration quoted in a fixture
    cannot end a block early. Masking preserves length and newlines, so the same
    offsets slice the original and the reported line numbers match the file.
    """
    if language == "go":
        return _delimited(masked, source, GO_TEST, GO_BLOCK_END, group=1)
    if language == "python":
        return _delimited(masked, source, PY_TEST, PY_BLOCK_END, group="name")
    return _sequential(masked, source, JS_TEST)


def _delimited(masked: str, source: str, opener: re.Pattern[str],
               closer: re.Pattern[str], group) -> list[Block]:
    """Blocks that end at the next declaration, as Go and Python ones do."""
    blocks: list[Block] = []
    for match in opener.finditer(masked):
        start = match.end()
        following = closer.search(masked, start)
        end = following.start() if following else len(masked)
        blocks.append(Block(match.group(group), masked[start:end], source[start:end],
                            _line_of(masked, match.start())))
    return blocks


def _sequential(masked: str, source: str, opener: re.Pattern[str]) -> list[Block]:
    """Blocks that end where the next one begins, as nested `it(` calls do."""
    matches = list(opener.finditer(masked))
    blocks: list[Block] = []
    for index, match in enumerate(matches):
        start = match.end()
        end = matches[index + 1].start() if index + 1 < len(matches) else len(masked)
        blocks.append(Block(match.group("name"), masked[start:end], source[start:end],
                            _line_of(masked, match.start())))
    return blocks


def _assert_count(language: str, body: str) -> int:
    pattern = {"go": GO_ASSERT, "python": PY_ASSERT, "js": JS_ASSERT}[language]
    return len(pattern.findall(body))


def _exempt(language: str, body: str) -> bool:
    pattern = {"go": GO_EXEMPT, "python": PY_EXEMPT, "js": JS_EXEMPT}[language]
    return bool(pattern.search(body))


def _allowed(text: str, block: Block) -> bool:
    """The marker may sit inside the block or on the lines just above it."""
    if ALLOW_MARKER in block.source:
        return True
    lines = text.splitlines()
    above = lines[max(0, block.line - 4) : block.line]
    return any(ALLOW_MARKER in line for line in above)


def _inspect(language: str, block: Block) -> str | None:
    body = block.text
    if _exempt(language, body):
        return None

    count = _assert_count(language, body)

    if count == 0:
        return (
            "asserts nothing, so it cannot fail when behavior changes. "
            "Assert the outcome, or delete the test."
        )

    if count == 1:
        if ENV_ONLY.search(body):
            return (
                "asserts only an environment variable, which tests the machine "
                "rather than the code. Move it to CI or test setup."
            )
        if MOCK_ONLY.search(body):
            return (
                "asserts only that a dependency was called, which passes for any "
                "pass-through wrapper. Assert the outcome the code produced."
            )
        if EXISTENCE.search(body) and DISCOVERY_NAME.search(block.name):
            return (
                "reads as path discovery: the name promises existence and the only "
                "assertion is an existence check. If a write is the behavior under "
                "test, say so in the name; otherwise move this to CI."
            )
        if HEALTH_NAME.search(block.name):
            return (
                "reads as a service or container health check, which tests the "
                "harness rather than the application. Start dependencies in setup "
                "and assert a domain outcome here."
            )
    return None


def main() -> int:
    payload = _read_payload()
    path = _target_file(payload)
    if path is None:
        return 0
    language = _language(path)
    if language is None:
        return 0

    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return 0

    findings: list[str] = []
    for block in _blocks(language, _mask_literals(language, text), text):
        if _allowed(text, block):
            continue
        problem = _inspect(language, block)
        if problem:
            findings.append(f"L{block.line} {block.name}: {problem}")

    if not findings:
        return 0

    detail = "\n".join(f"  - {item}" for item in findings)
    report = (
        f"core-logic-test-guard flagged {len(findings)} low-value test(s) in {path.name}:\n"
        f"{detail}\n"
        "See the core-logic rules in .ai-agents/skills/test-driven-development/SKILL.md. "
        f"Mark a deliberate exception with a `{ALLOW_MARKER}` comment."
    )

    # Both fields on purpose. systemMessage reaches the person; additionalContext
    # reaches the model, which is the one that has to fix the test. Whichever the
    # installed host does not support is ignored, and a guard nobody can act on
    # is not worth firing.
    print(
        json.dumps(
            {
                "systemMessage": report,
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": report,
                },
            }
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
