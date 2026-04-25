#!/bin/sh
# squad SessionEnd hook — drop notify_endpoints rows for this instance so
# senders stop dialing a port that's already gone. Default ON when stop_listen
# is enabled. SQUAD_NO_HOOKS=1 disables.

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
    exit 0
fi
INSTANCE="session-$SUFFIX"

if command -v gtimeout >/dev/null 2>&1; then
    gtimeout 2s "$SQUAD_BIN" notify-cleanup --instance "$INSTANCE" >/dev/null 2>&1
elif command -v timeout >/dev/null 2>&1; then
    timeout 2s "$SQUAD_BIN" notify-cleanup --instance "$INSTANCE" >/dev/null 2>&1
else
    "$SQUAD_BIN" notify-cleanup --instance "$INSTANCE" >/dev/null 2>&1
fi
exit 0
