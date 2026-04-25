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
    "git commit"*|*"&& git commit"*|*"; git commit"*|*"| git commit"*) ;;
    *) exit 0 ;;
esac

PATTERN='\b(BUG|FEAT|TASK|CHORE|DEBT|BET|STD|OPS)-[0-9]+\b'

# Extract candidate message text from the command. Three forms in the wild:
#   git commit -m "msg"        (quoted)
#   git commit -m msg          (unquoted single-token)
#   git commit -F path         (file)
# All three are scanned. Heredoc / $(cat <<EOF...) forms expand at runtime
# and never reach this hook as static text — those are caught by the staged-
# diff scan downstream and (more reliably) by COMMIT_EDITMSG which git
# populates before pre-commit hooks run.
MSG_QUOTED=$(printf '%s' "$CMD" \
    | sed -n "s/.*-m[[:space:]]*['\"]\([^'\"]*\)['\"].*/\1/p" | head -n1)
MSG_UNQUOTED=$(printf '%s' "$CMD" \
    | sed -n 's/.*-m[[:space:]]*\([^[:space:]"'"'"']*\).*/\1/p' | head -n1)
MSG_FROM_FILE=""
MSG_FROM_FILE_PATH=$(printf '%s' "$CMD" \
    | sed -n 's/.*-F[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p' | head -n1)
if [ -n "$MSG_FROM_FILE_PATH" ] && [ -f "$MSG_FROM_FILE_PATH" ]; then
    MSG_FROM_FILE=$(head -c 65536 "$MSG_FROM_FILE_PATH" 2>/dev/null || printf '')
fi

GITDIR_MSG=""
if [ -n "$(git rev-parse --git-dir 2>/dev/null)" ]; then
    GITDIR=$(git rev-parse --git-dir)
    if [ -f "$GITDIR/COMMIT_EDITMSG" ]; then
        # Cap at 64KB to bound the worst case.
        GITDIR_MSG=$(head -c 65536 "$GITDIR/COMMIT_EDITMSG" 2>/dev/null || printf '')
    fi
fi

FOUND=""

# Scan every candidate source.
for CANDIDATE in "$MSG_QUOTED" "$MSG_UNQUOTED" "$MSG_FROM_FILE" "$GITDIR_MSG"; do
    [ -z "$CANDIDATE" ] && continue
    HIT=$(printf '%s\n' "$CANDIDATE" | grep -E "$PATTERN" || true)
    if [ -n "$HIT" ]; then
        FOUND="${FOUND}message: ${HIT}\n"
        break
    fi
done

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
