#!/bin/sh
# squad pre-edit touch-conflict warning. Default OFF. Always exits 0.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

FILE=$(printf '%s' "${TOOL_INPUT:-}" \
    | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
[ -z "$FILE" ] && exit 0

SQUAD_BIN="${SQUAD_BIN:-squad}"
command -v "$SQUAD_BIN" >/dev/null 2>&1 || exit 0

TOUCHES=$("$SQUAD_BIN" touches list-others --json 2>/dev/null || printf '[]')

# Split JSON array entries one-per-line (each ends with `}`). Then keep only
# entries whose "path" field EXACTLY equals $FILE — substring match against the
# whole blob falsely warned for paths like "g", ":", or "/tmp/foo" matching
# inside "/tmp/foo.go".
ENTRIES=$(printf '%s' "$TOUCHES" | sed 's/},[[:space:]]*{/}\n{/g')
MATCH_LINE=$(printf '%s\n' "$ENTRIES" \
    | grep -F "\"path\":\"$FILE\"" | head -n1)
[ -z "$MATCH_LINE" ] && exit 0

OWNER=$(printf '%s' "$MATCH_LINE" \
    | sed -n 's/.*"agent_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
REPO=$(printf '%s' "$MATCH_LINE" \
    | sed -n 's/.*"repo"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)

[ -z "$OWNER" ] && OWNER="another agent"
[ -z "$REPO" ] && REPO="this repo"

printf 'squad: %s is touching %s in %s — `squad knock @%s` to coordinate.\n' \
    "$OWNER" "$FILE" "$REPO" "$OWNER" 1>&2
exit 0
