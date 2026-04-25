#!/bin/sh
# squad pre-commit tick check. Default OFF.
# Reads $TOOL_INPUT (JSON from Claude Code); blocks if last tick > 5 min.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

CMD=$(printf '%s' "${TOOL_INPUT:-}" \
    | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)

# Match `git commit` only when it's the actual command being run, not arbitrary
# substrings (`echo "git commit"` shouldn't trigger the gate). Accept leading
# whitespace, optional `git` env-var prefix, or pipeline-trailing forms.
case "$CMD" in
    "git commit"*|*"&& git commit"*|*"; git commit"*|*"| git commit"*) ;;
    *) exit 0 ;;
esac

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

_json_num() {
    if command -v jq >/dev/null 2>&1; then
        jq -r ".$1 // empty" 2>/dev/null
    else
        sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\\([0-9]*\\).*/\\1/p" | head -n1
    fi
}

LAST_TICK=$(_squad_run whoami --json 2>/dev/null | _json_num last_tick_at)
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
