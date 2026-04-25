#!/bin/sh
# squad Stop hook — auto-handoff a stale open claim. Default OFF. Always exits 0.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
command -v "$SQUAD_BIN" >/dev/null 2>&1 || exit 0

WHOAMI=$("$SQUAD_BIN" whoami --json 2>/dev/null || printf '{}')

CLAIM_ID=$(printf '%s' "$WHOAMI" \
    | sed -n 's/.*"item_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
[ -z "$CLAIM_ID" ] && exit 0

LAST_TOUCH=$(printf '%s' "$WHOAMI" \
    | sed -n 's/.*"last_touch"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' | head -n1)
[ -z "$LAST_TOUCH" ] && exit 0

NOW=$(date +%s)
DELTA=$((NOW - LAST_TOUCH))
THRESHOLD=1800

[ "$DELTA" -lt "$THRESHOLD" ] && exit 0

"$SQUAD_BIN" handoff "session ended" >/dev/null 2>&1 || true
exit 0
