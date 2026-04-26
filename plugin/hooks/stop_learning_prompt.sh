#!/bin/sh
# squad Stop hook — at end of session, if the working tree has non-trivial
# net change and no learning has been proposed for this session, prompt
# the agent to file one. Default OFF; opt in via squad install_hooks.
# Always exits 0 so Claude Code never blocks on hook failure.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
command -v "$SQUAD_BIN" >/dev/null 2>&1 || exit 0

_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 5s "$SQUAD_BIN" "$@"
    elif command -v timeout >/dev/null 2>&1; then
        timeout 5s "$SQUAD_BIN" "$@"
    else
        "$SQUAD_BIN" "$@"
    fi
}

if ! git rev-parse --git-dir >/dev/null 2>&1; then
    exit 0
fi
EMPTY_TREE=$(git hash-object -t tree /dev/null 2>/dev/null)
if git rev-parse --verify HEAD >/dev/null 2>&1; then
    DIFF=$(git diff --numstat HEAD 2>/dev/null)
else
    DIFF=$(git diff --numstat "$EMPTY_TREE" 2>/dev/null)
fi

VERDICT=$(printf '%s\n' "$DIFF" | _squad_run learning triviality-check 2>/dev/null)
[ "$VERDICT" = "non-trivial" ] || exit 0

SESSION="${SQUAD_SESSION_ID:-${TERM_SESSION_ID:-${TMUX_PANE:-}}}"
if [ -n "$SESSION" ]; then
    if _squad_run learning list --state proposed 2>/dev/null | grep -q "$SESSION" 2>/dev/null; then
        exit 0
    fi
fi

cat <<'PROMPT'
[squad] You touched non-trivial code this session. Before signing off, propose at least one learning artifact:

  squad learning propose gotcha   <slug> --title "..." --area <area>   # this looked like X but is Y
  squad learning propose pattern  <slug> --title "..." --area <area>   # we do it this way here
  squad learning propose dead-end <slug> --title "..." --area <area>   # we tried X, it didn't work because Y

A human will approve or reject. Approved learnings auto-load for future sessions touching matching paths.
PROMPT

exit 0
