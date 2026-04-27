#!/bin/sh
# squad subagent/task hook — record SubagentStart/Stop and TaskCreated/Completed
# events. Posts to chat (existing) AND records an agent_events row for
# SubagentStart/Stop only (additive). Default ON; SQUAD_NO_HOOKS=1 disables.
# Always exits 0; never blocks Claude.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

PAYLOAD=$(cat)

_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 2s "$SQUAD_BIN" "$@" >/dev/null 2>&1
    elif command -v timeout >/dev/null 2>&1; then
        timeout 2s "$SQUAD_BIN" "$@" >/dev/null 2>&1
    else
        "$SQUAD_BIN" "$@" >/dev/null 2>&1
    fi
}

printf '%s' "$PAYLOAD" | _squad_run subagent-event

[ -z "$PAYLOAD" ] && exit 0

if command -v jq >/dev/null 2>&1; then
    HOOK_NAME=$(printf '%s' "$PAYLOAD" | jq -r '.hook_event_name // empty' 2>/dev/null)
    AGENT_TYPE=$(printf '%s' "$PAYLOAD" | jq -r '.agent_type // empty' 2>/dev/null)
    DESCRIPTION=$(printf '%s' "$PAYLOAD" | jq -r '.description // empty' 2>/dev/null)
    EXIT_CODE=$(printf '%s' "$PAYLOAD" | jq -r '.exit_code // 0' 2>/dev/null)
else
    # sed fallback assumes Claude Code's flat envelope shape. Nested keys
    # would match the inner occurrence due to leading .* greediness — fine
    # in practice, latent if the envelope shape ever nests these fields.
    HOOK_NAME=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"hook_event_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    AGENT_TYPE=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"agent_type"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    DESCRIPTION=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"description"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    EXIT_CODE=$(printf '%s' "$PAYLOAD" \
        | sed -n 's/.*"exit_code"[[:space:]]*:[[:space:]]*\([0-9-]*\).*/\1/p' | head -n1)
    [ -z "$EXIT_CODE" ] && EXIT_CODE=0
fi

case "$HOOK_NAME" in
    SubagentStart) KIND=subagent_start ;;
    SubagentStop)  KIND=subagent_stop ;;
    *) exit 0 ;;
esac

TOOL_ARG="$AGENT_TYPE"
[ -z "$TOOL_ARG" ] && TOOL_ARG="$DESCRIPTION"

_squad_run event record \
    --kind "$KIND" \
    --tool "$TOOL_ARG" \
    --target "$DESCRIPTION" \
    --exit "$EXIT_CODE"

exit 0
