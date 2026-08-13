#!/bin/sh
# Installs the pre-commit gate for contributors to this toolkit.
#
# Opt-in and repo-local. link-ai-agents.sh is not the place for it: that script
# runs inside consumer repositories, and a gate for developing the toolkit
# firing on someone else's commits would be a surprise rather than a service.
#
# Written into .git/hooks/ rather than switched on with core.hooksPath, because
# link-ai-agents.sh already installs prepare-commit-msg there and moving the
# hook path would silently stop that one running.
#
# Usage: sh scripts/install-dev-hooks.sh [--uninstall]
set -eu

root=$(git rev-parse --show-toplevel 2>/dev/null) || {
    echo "not a git repository; nothing to install" >&2
    exit 1
}
hook="$root/.git/hooks/pre-commit"
marker="# vibe-agent dev hook"

if [ "${1:-}" = "--uninstall" ]; then
    if [ -f "$hook" ] && grep -q "$marker" "$hook"; then
        rm "$hook"
        echo "removed $hook"
    else
        echo "no vibe-agent pre-commit hook at $hook"
    fi
    exit 0
fi

# Refuse to clobber someone else's hook. A silent overwrite of a hook this
# script did not write is the kind of help nobody asked for.
if [ -f "$hook" ] && ! grep -q "$marker" "$hook"; then
    echo "a pre-commit hook already exists at $hook and was not installed by this script." >&2
    echo "inspect it, then remove it and re-run if you want ours." >&2
    exit 1
fi

cat > "$hook" <<EOF
#!/bin/sh
$marker - installed by scripts/install-dev-hooks.sh
# Body lives in scripts/pre-commit.sh so it is tracked and reviewable.
exec sh "$root/scripts/pre-commit.sh"
EOF
chmod +x "$hook"

echo "installed $hook"
echo "it runs gofmt, vet, golangci-lint, and the asset checks on what you stage."
echo "bypass one commit with: git commit --no-verify"
