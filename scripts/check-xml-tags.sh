#!/usr/bin/env sh
# Fail when a file uses an XML section tag wrongly.
#
# The tags partition rules so a model can address one block at a time. An unclosed
# tag swallows every section after it, which is silent: the file still renders and
# every other check still passes. Only the model's reading changes, which is the
# one thing no other check in this repo looks at.
#
# Fenced code blocks are skipped. The a11y and frontend references contain <div>,
# <dialog>, and <picture> in examples, and a checker that flags those is a checker
# somebody turns off.
#
# Checks: fence awareness, balance, membership in the documented set, position
# relative to YAML frontmatter, and nesting. Tag set and rationale:
# .ai-agents/AUTHORING.md, "XML section tags".
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"

# Charter tags, then asset tags. Kept in sync with the tables in AUTHORING.md.
KNOWN="scope precedence always_on delivery_gates claude_specific other_harnesses persona prerequisites context required rules procedure inputs outputs verification antipatterns routing escalation"

scan() {
  awk -v known="$KNOWN" -v rel="$1" '
    BEGIN {
      n = split(known, list, " ")
      for (i = 1; i <= n; i++) valid[list[i]] = 1
      open = ""; openline = 0; fence = 0; bad = 0
    }
    # Fenced code: ``` or ~~~ toggles. Everything inside is an example.
    /^[ \t]*(```|~~~)/ { fence = !fence; next }
    fence { next }

    /^<[a-z_]+>$/ {
      tag = substr($0, 2, length($0) - 2)
      if (!(tag in valid)) {
        printf "%s:%d: <%s> is not in the documented tag set\n", rel, NR, tag
        bad = 1
        next
      }
      if (NR == 1) {
        printf "%s:1: a tag is the first line; YAML frontmatter must come first\n", rel
        bad = 1
      }
      if (open != "") {
        printf "%s:%d: <%s> opens while <%s> (line %d) is still open; tags do not nest\n", \
          rel, NR, tag, open, openline
        bad = 1
      }
      open = tag; openline = NR
      next
    }

    /^<\/[a-z_]+>$/ {
      tag = substr($0, 3, length($0) - 3)
      if (!(tag in valid)) {
        printf "%s:%d: </%s> is not in the documented tag set\n", rel, NR, tag
        bad = 1
        next
      }
      if (open == "") {
        printf "%s:%d: </%s> closes but nothing is open\n", rel, NR, tag
        bad = 1
      } else if (open != tag) {
        printf "%s:%d: </%s> closes but <%s> (line %d) is open\n", rel, NR, tag, open, openline
        bad = 1
      }
      open = ""
      next
    }

    END {
      if (open != "") {
        printf "%s:%d: <%s> is never closed\n", rel, openline, open
        bad = 1
      }
      exit bad
    }
  ' "$2"
}

problems=0
for path in "$root/AGENTS.md" "$root/CLAUDE.md" "$root/CURSOR.md" \
            $(find "$root/.ai-agents" -name "*.md" 2>/dev/null | sort); do
  [ -f "$path" ] || continue
  scan "${path#"$root"/}" "$path" || problems=$((problems + 1))
done

if [ "$problems" -ne 0 ]; then
  echo ""
  echo "$problems file(s) with tag problems. See .ai-agents/AUTHORING.md, \"XML section tags\"."
  exit 1
fi

echo "check-xml-tags: OK"
