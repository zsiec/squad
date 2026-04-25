#!/bin/sh
# squad pre-commit PM-trace check. Default OFF.
# Greps `git diff --staged` and the commit message for backlog ID patterns.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

CMD=$(printf '%s' "${TOOL_INPUT:-}" \
    | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)

case "$CMD" in
    *"git commit"*) ;;
    *) exit 0 ;;
esac

PATTERN='\b(BUG|FEAT|TASK|CHORE|DEBT|BET|STD|OPS)-[0-9]+\b'

MSG=$(printf '%s' "$CMD" \
    | sed -n "s/.*-m[[:space:]]*['\"]\([^'\"]*\)['\"].*/\1/p" | head -n1)

if [ -z "$MSG" ] && [ -n "$(git rev-parse --git-dir 2>/dev/null)" ]; then
    GITDIR=$(git rev-parse --git-dir)
    if [ -f "$GITDIR/COMMIT_EDITMSG" ]; then
        MSG=$(cat "$GITDIR/COMMIT_EDITMSG" 2>/dev/null || printf '')
    fi
fi

FOUND=""

if [ -n "$MSG" ]; then
    HIT=$(printf '%s\n' "$MSG" | grep -E "$PATTERN" || true)
    [ -n "$HIT" ] && FOUND="${FOUND}message: ${HIT}\n"
fi

if command -v git >/dev/null 2>&1; then
    DIFF_HIT=$(git diff --staged --no-color 2>/dev/null \
        | grep -E "^\+" | grep -Ev "^\+\+\+ " | grep -E "$PATTERN" || true)
    [ -n "$DIFF_HIT" ] && FOUND="${FOUND}staged diff:\n${DIFF_HIT}\n"
fi

if [ -n "$FOUND" ]; then
    printf 'squad: backlog ID(s) found in commit — squad enforces no-PM-traces.\n%b' "$FOUND" 1>&2
    exit 1
fi
exit 0
