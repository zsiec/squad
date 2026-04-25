#!/bin/sh
# squad Stop hook — auto-handoff a stale open claim. Default OFF. Always exits 0.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
command -v "$SQUAD_BIN" >/dev/null 2>&1 || exit 0

# Cap every squad subprocess so a regression that hangs cannot freeze the
# Claude Code session. Falls back to bare invocation if no timeout binary.
_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 2s "$SQUAD_BIN" "$@"
    elif command -v timeout >/dev/null 2>&1; then
        timeout 2s "$SQUAD_BIN" "$@"
    else
        "$SQUAD_BIN" "$@"
    fi
}

# Extract a JSON field robustly. jq if available; sed fallback for portability.
_json_str() {
    if command -v jq >/dev/null 2>&1; then
        jq -r ".$1 // empty" 2>/dev/null
    else
        sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" | head -n1
    fi
}

_json_num() {
    if command -v jq >/dev/null 2>&1; then
        jq -r ".$1 // empty" 2>/dev/null
    else
        sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\\([0-9]*\\).*/\\1/p" | head -n1
    fi
}

WHOAMI=$(_squad_run whoami --json 2>/dev/null || printf '{}')

CLAIM_ID=$(printf '%s' "$WHOAMI" | _json_str item_id)
[ -z "$CLAIM_ID" ] && exit 0

LAST_TOUCH=$(printf '%s' "$WHOAMI" | _json_num last_touch)
[ -z "$LAST_TOUCH" ] && exit 0

NOW=$(date +%s)
DELTA=$((NOW - LAST_TOUCH))
THRESHOLD=1800

[ "$DELTA" -lt "$THRESHOLD" ] && exit 0

_squad_run handoff --in-flight "$CLAIM_ID" --note "session ended (auto-handoff after ${DELTA}s of no activity)" >/dev/null 2>&1 || true
exit 0
