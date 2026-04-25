#!/bin/sh
# squad PostToolUse hook — mid-turn mailbox flush. After every tool call,
# emit any pending peer chat as hookSpecificOutput.additionalContext so
# Claude sees it on the next reasoning step without waiting for Stop.
# Default ON. SQUAD_NO_HOOKS=1 disables.

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

_squad_run mailbox --format additional-context --event PostToolUse 2>/dev/null
exit 0
