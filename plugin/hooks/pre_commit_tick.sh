#!/bin/sh
# squad pre-commit tick check. Default OFF.
# Reads $TOOL_INPUT (JSON from Claude Code); blocks if last tick > 5 min.

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

SQUAD_BIN="${SQUAD_BIN:-squad}"
command -v "$SQUAD_BIN" >/dev/null 2>&1 || exit 0

LAST_TICK=$("$SQUAD_BIN" whoami --json 2>/dev/null \
    | sed -n 's/.*"last_tick_at"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' | head -n1)
[ -z "$LAST_TICK" ] && exit 0

NOW=$(date +%s)
DELTA=$((NOW - LAST_TICK))
THRESHOLD=300

if [ "$DELTA" -gt "$THRESHOLD" ]; then
    printf 'squad: last tick was %ss ago (>%ss). Run `squad tick` first — peers may have posted heads-ups.\n' \
        "$DELTA" "$THRESHOLD" 1>&2
    exit 1
fi
exit 0
