#!/bin/sh
# squad UserPromptSubmit hook — mailbox flush at every prompt boundary.
# The Stop and PostToolUse hooks handle wake/mid-turn delivery; this hook
# is the belt-and-braces fallback so any miss is caught at the next user
# prompt regardless of transport. Default ON. SQUAD_NO_HOOKS=1 disables.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 2s "$SQUAD_BIN" "$@"
    elif command -v timeout >/dev/null 2>&1; then
        timeout 2s "$SQUAD_BIN" "$@"
    else
        "$SQUAD_BIN" "$@"
    fi
}

_squad_run mailbox --format additional-context --event UserPromptSubmit 2>/dev/null
exit 0
