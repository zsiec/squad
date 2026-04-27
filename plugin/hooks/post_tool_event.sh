#!/bin/sh
# squad PostToolUse hook — record a post_tool event into agent_events with
# exit code + duration when the envelope exposes them. Always exits 0;
# recorder failures must not block the parent agent. SQUAD_NO_HOOKS=1
# disables. SQUAD_EVENTS_FILTER_READ=1 skips Read events.

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

EXIT_CODE=0
DURATION_MS=0

if command -v jq >/dev/null 2>&1; then
    TOOL=$(printf '%s' "$PAYLOAD" | jq -r '.tool_name // empty' 2>/dev/null)
    TARGET=$(printf '%s' "$PAYLOAD" \
        | jq -r '.tool_input.command // .tool_input.file_path // .tool_input.path // .tool_input.url // .tool_input.pattern // empty' 2>/dev/null)
    EC=$(printf '%s' "$PAYLOAD" | jq -r '.tool_response.exit_code // .exit_code // 0' 2>/dev/null)
    case "$EC" in
        ''|*[!0-9-]*) ;;
        *) EXIT_CODE="$EC" ;;
    esac
    DUR=$(printf '%s' "$PAYLOAD" | jq -r '.tool_response.duration_ms // .duration_ms // 0' 2>/dev/null)
    case "$DUR" in
        ''|*[!0-9]*) ;;
        *) DURATION_MS="$DUR" ;;
    esac
else
    TOOL=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"tool_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    # -E for portable alternation: BSD sed treats \| as a literal pipe, so
    # the basic-RE form silently emits empty TARGET on macOS.
    TARGET=$(printf '%s' "$PAYLOAD" \
        | sed -nE 's/.*"(command|file_path|path|url|pattern)"[[:space:]]*:[[:space:]]*"([^"]*)".*/\2/p' | head -n1)
    EC=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"exit_code"[[:space:]]*:[[:space:]]*\([0-9-]*\).*/\1/p' | head -n1)
    case "$EC" in
        ''|*[!0-9-]*) ;;
        *) EXIT_CODE="$EC" ;;
    esac
    DUR=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"duration_ms"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' | head -n1)
    case "$DUR" in
        ''|*[!0-9]*) ;;
        *) DURATION_MS="$DUR" ;;
    esac
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

_squad_run event record \
    --kind post_tool \
    --tool "$TOOL" \
    --target "$TARGET" \
    --exit "$EXIT_CODE" \
    --duration-ms "$DURATION_MS" \
    >/dev/null 2>&1
exit 0
