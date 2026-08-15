#!/bin/sh
# Pre-commit gate for people working ON this toolkit.
#
# Deliberately in scripts/ and not in .ai-agents/. That tree is shipped: it is
# linked into consumer repositories and copied to ~/.vibe-agent by
# install-global.sh. A gate for developing the toolkit has no business firing in
# a repository that merely uses it, and would be a surprise there rather than a
# service.
#
# Install with `sh scripts/install-dev-hooks.sh`. Nothing installs it for you,
# for the same reason.
#
# Every check calls a Makefile target or a repository script. Those are where a
# command is decided; this file decides only when to run one. Spelling the
# commands out here would put a second copy in front of every commit, and the
# copy that drifts is the one that lets the bad commit through.
#
# Bypass one commit with `git commit --no-verify` when you know why. CI runs the
# full set regardless, so a bypass costs a red build rather than a silent gap.
set -eu

cd "$(git rev-parse --show-toplevel)"

staged() {
    git diff --cached --name-only --diff-filter=ACMR -- "$@"
}

# An amend on a clean tree stages nothing and has nothing to check.
if git diff --cached --quiet --diff-filter=ACMR; then
    exit 0
fi

# Go: gofmt first, because it is the cheapest check and the one most likely to
# fire. Reported, never applied, so the fix stays a deliberate act by the author
# instead of a surprise inside the commit they are writing.
if [ -n "$(staged 'runtime/**/*.go' 'runtime/*.go')" ]; then
    unformatted=$(cd runtime && gofmt -l .)
    if [ -n "$unformatted" ]; then
        echo "pre-commit: not gofmt clean:" >&2
        echo "$unformatted" | sed 's|^|  runtime/|' >&2
        echo "run: make -C runtime fmt" >&2
        exit 1
    fi
    make -C runtime vet
    make -C runtime golangci
fi

# Assets: the routers and the generated harness views go stale without anyone
# noticing, because nothing fails until a session reads a command file that no
# longer matches its source.
if [ -n "$(staged '.ai-agents' '.claude' '.cursor' '.codex' '.opencode')" ]; then
    sh scripts/check-ai-agents-routers.sh
    sh scripts/check-generated-views.sh
    sh scripts/check-workspace-install.sh
    sh scripts/check-xml-tags.sh
fi

# Remaining Python hook scripts may own local contract tests. The runtime-owned
# guards are tested by the Go suite under runtime/.
if [ -n "$(staged '.ai-agents/hooks')" ]; then
    for suite in .ai-agents/hooks/*-test.py; do
        [ -e "$suite" ] || continue
        python3 "$suite"
    done
fi

# `make -C runtime test` is deliberately absent. It is the slow gate, it belongs
# to CI, and a pre-commit hook that takes a minute is one people learn to skip.
exit 0
