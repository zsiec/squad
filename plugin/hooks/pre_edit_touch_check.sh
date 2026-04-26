#!/bin/sh
# squad pre-edit touch-conflict check. Defers to `squad touches policy <file>`
# so warn/deny mode and the JSON shape live in Go where they are tested.
# Always exits 0; Claude Code reads the JSON on stdout for the actual decision.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

FILE=$(printf '%s' "${TOOL_INPUT:-}" \
    | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
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
