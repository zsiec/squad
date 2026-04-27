#!/bin/sh
# squad pre-commit AGENTS.md drift check.
#
# AGENTS.md is generated from the ledger by `squad scaffold agents-md`.
# CLAUDE.md is the only hand-edited contract file. This hook fires
# before `git commit` invocations, runs `squad scaffold agents-md
# --check`, and refuses the commit when the staged AGENTS.md drifts
# from the current generator output.
#
# Same shape as pre_commit_pm_traces.sh: PreToolUse delivers the event
# payload via stdin as JSON; we parse tool_name and tool_input.command.
# Skip when the squad binary is not on PATH so a misconfigured plugin
# does not block legitimate commits.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

PAYLOAD=$(cat)
[ -z "$PAYLOAD" ] && exit 0

if command -v jq >/dev/null 2>&1; then
    TOOL=$(printf '%s' "$PAYLOAD" | jq -r '.tool_name // empty' 2>/dev/null)
    CMD=$(printf '%s' "$PAYLOAD" | jq -r '.tool_input.command // empty' 2>/dev/null)
else
    TOOL=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"tool_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    CMD=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
fi

[ "$TOOL" = "Bash" ] || exit 0

case "$CMD" in
    "git commit"*|*"&& git commit"*|*"; git commit"*|*"| git commit"*) ;;
    *) exit 0 ;;
esac

# Only fire when AGENTS.md is part of the staged set — a commit that
# does not touch AGENTS.md should not be blocked by drift in someone
# else's tree state.
if command -v git >/dev/null 2>&1; then
    STAGED=$(git diff --staged --name-only 2>/dev/null || true)
    case "$STAGED" in
        *AGENTS.md*) ;;
        *) exit 0 ;;
    esac
else
    exit 0
fi

if ! command -v squad >/dev/null 2>&1; then
    exit 0
fi

if OUT=$(squad scaffold agents-md --check 2>&1); then
    exit 0
fi

printf 'squad: AGENTS.md drift detected — regenerate before commit.\n%s\n' "$OUT" 1>&2
printf 'fix: squad scaffold agents-md\n' 1>&2
exit 1
