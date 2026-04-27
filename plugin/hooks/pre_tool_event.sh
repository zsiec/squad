#!/bin/sh
# squad PreToolUse hook — record a pre_tool event into agent_events.
# Always exits 0; recorder failures must not block the parent agent.
# SQUAD_NO_HOOKS=1 disables. SQUAD_EVENTS_FILTER_READ=1 skips Read events
# (cheap, frequent, low signal).

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

PAYLOAD=$(cat)
[ -z "$PAYLOAD" ] && exit 0

if command -v jq >/dev/null 2>&1; then
    TOOL=$(printf '%s' "$PAYLOAD" | jq -r '.tool_name // empty' 2>/dev/null)
    TARGET=$(printf '%s' "$PAYLOAD" \
        | jq -r '.tool_input.command // .tool_input.file_path // .tool_input.path // .tool_input.url // .tool_input.pattern // empty' 2>/dev/null)
else
    TOOL=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"tool_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    # -E for portable alternation: BSD sed treats \| as a literal pipe, so
    # the basic-RE form silently emits empty TARGET on macOS.
    TARGET=$(printf '%s' "$PAYLOAD" \
        | sed -nE 's/.*"(command|file_path|path|url|pattern)"[[:space:]]*:[[:space:]]*"([^"]*)".*/\2/p' | head -n1)
fi

if [ "${SQUAD_EVENTS_FILTER_READ:-0}" = "1" ] && [ "$TOOL" = "Read" ]; then
    exit 0
fi

TARGET=$(printf '%s' "$TARGET" | tr -d '\r\n' | cut -c1-200)

_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 2s "$SQUAD_BIN" "$@"
    elif command -v timeout >/dev/null 2>&1; then
        timeout 2s "$SQUAD_BIN" "$@"
    else
        "$SQUAD_BIN" "$@"
    fi
}

_squad_run event record --kind pre_tool --tool "$TOOL" --target "$TARGET" >/dev/null 2>&1
exit 0
