#!/bin/sh
# squad session-start hook — auto-register and inject context. Default ON.
# SQUAD_NO_HOOKS=1 disables; SQUAD_BIN overrides the binary lookup.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

# Cap every squad subprocess so a regression that hangs (or an SQUAD_BIN
# pointing at /usr/bin/yes) cannot freeze the Claude Code session.
_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 2s "$SQUAD_BIN" "$@"
    elif command -v timeout >/dev/null 2>&1; then
        timeout 2s "$SQUAD_BIN" "$@"
    else
        "$SQUAD_BIN" "$@"
    fi
}

# Extract a string field from a JSON blob on stdin. Uses jq if available
# (handles escaped quotes correctly); falls back to a regex that's good
# enough for squad's own --json outputs (no nested escapes in id/repo fields).
_json_str() {
    if command -v jq >/dev/null 2>&1; then
        jq -r ".$1 // empty" 2>/dev/null
    else
        sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" | head -n1
    fi
}

# Extract a numeric field, similarly.
_json_num() {
    if command -v jq >/dev/null 2>&1; then
        jq -r ".$1 // empty" 2>/dev/null
    else
        sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\\([0-9]*\\).*/\\1/p" | head -n1
    fi
}

_squad_run register --no-repo-check >/dev/null 2>&1 || true

WHOAMI_ID=$(_squad_run whoami --json 2>/dev/null | _json_str id)
REPO_NAME=$(_squad_run workspace status --json 2>/dev/null | _json_str current_repo)
if command -v jq >/dev/null 2>&1; then
    NEXT_TOP=$(_squad_run next --json --limit 3 2>/dev/null \
        | jq -r '.[].id' 2>/dev/null \
        | head -n3 | tr '\n' ',' | sed 's/,$//')
else
    NEXT_TOP=$(_squad_run next --json --limit 3 2>/dev/null \
        | grep -oE '"id":[[:space:]]*"[^"]+"' \
        | sed -E 's/"id":[[:space:]]*"([^"]+)"/\1/' \
        | head -n3 | tr '\n' ',' | sed 's/,$//')
fi

[ -z "$WHOAMI_ID" ] && exit 0
[ -z "$NEXT_TOP" ] && NEXT_TOP="(none)"
[ -z "$REPO_NAME" ] && REPO_NAME="(no repo)"

printf '[squad] registered as %s in %s; ready stack top: %s.\n' \
    "$WHOAMI_ID" "$REPO_NAME" "$NEXT_TOP"
exit 0
