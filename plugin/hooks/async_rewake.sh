#!/bin/sh
# squad asyncRewake hook — long-poll the mailbox in the background while the
# session is idle. Exits 2 with a stderr reason when a peer message arrives;
# Claude Code injects stderr as a system reminder mid-turn. Default OFF
# (opt-in) until the asyncRewake hook contract stabilizes.
# SQUAD_NO_HOOKS=1 disables.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

SESSION_KEY="${SQUAD_SESSION_ID:-${TERM_SESSION_ID:-${ITERM_SESSION_ID:-${TMUX_PANE:-${WT_SESSION:-}}}}}"
if [ -n "$SESSION_KEY" ]; then
    SUFFIX=$(printf '%s' "$SESSION_KEY" | shasum 2>/dev/null | cut -c1-12)
    [ -z "$SUFFIX" ] && SUFFIX=$(printf '%s' "$SESSION_KEY" | cut -c1-12)
else
    SUFFIX=$(openssl rand -hex 6 2>/dev/null || printf '%s' "$$" | cut -c1-12)
fi
INSTANCE="rewake-$SUFFIX"

# Emit decision-block JSON to stdout AND mirror it to stderr; asyncRewake
# uses stderr (or stdout when stderr empty) as the system-reminder body.
OUT=$("$SQUAD_BIN" listen --instance "$INSTANCE" --max 30m --fallback 30s 2>/dev/null)
RC=$?
if [ "$RC" = "2" ] && [ -n "$OUT" ]; then
    printf '%s\n' "$OUT" >&2
    exit 2
fi
exit 0
