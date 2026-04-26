#!/bin/sh
# squad pre-edit touch-conflict check. Defers to `squad touches policy <file>`
# so warn/deny mode and the JSON shape live in Go where they are tested.
# Always exits 0; Claude Code reads the JSON on stdout for the actual decision.
#
# PreToolUse delivers its event payload via stdin as JSON, never via env vars.
# We read the whole payload, then extract tool_name and tool_input.file_path.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

PAYLOAD=$(cat)
[ -z "$PAYLOAD" ] && exit 0

if command -v jq >/dev/null 2>&1; then
    TOOL=$(printf '%s' "$PAYLOAD" | jq -r '.tool_name // empty' 2>/dev/null)
    FILE=$(printf '%s' "$PAYLOAD" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
else
    TOOL=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"tool_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    FILE=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
fi

case "$TOOL" in
    Edit|Write|MultiEdit) ;;
    *) exit 0 ;;
esac

[ -z "$FILE" ] && exit 0

SQUAD_BIN="${SQUAD_BIN:-squad}"
command -v "$SQUAD_BIN" >/dev/null 2>&1 || exit 0

_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 2s "$SQUAD_BIN" "$@"
    elif command -v timeout >/dev/null 2>&1; then
        timeout 2s "$SQUAD_BIN" "$@"
    else
        "$SQUAD_BIN" "$@"
    fi
}

OUT=$(_squad_run touches policy "$FILE" 2>/dev/null)
[ -z "$OUT" ] && exit 0
printf '%s\n' "$OUT"
exit 0
