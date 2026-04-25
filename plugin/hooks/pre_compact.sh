#!/bin/sh
# squad PreCompact hook — survive context compaction with coordination state intact.
# When Claude is about to compact context, dump the agent's current claim,
# active file touches, and last few chat lines as additionalContext so the
# post-compact agent still knows what it's working on. Default ON.
# SQUAD_NO_HOOKS=1 disables.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

# Cap every squad subprocess so a regression that hangs cannot freeze
# the Claude Code session at compact time.
_squad_run() {
    if command -v gtimeout >/dev/null 2>&1; then
        gtimeout 2s "$SQUAD_BIN" "$@"
    elif command -v timeout >/dev/null 2>&1; then
        timeout 2s "$SQUAD_BIN" "$@"
    else
        "$SQUAD_BIN" "$@"
    fi
}

# whoami emits {id, claim, intent, last_tick_at, ...}; touches list-others
# is the wrong direction (peers) so we ask the binary directly for our
# own active touches via the JSON listing on `touches` (no flag → caller's).
WHOAMI=$(_squad_run whoami --json 2>/dev/null)
if [ -z "$WHOAMI" ]; then
    exit 0
fi

if command -v jq >/dev/null 2>&1; then
    AGENT=$(printf '%s' "$WHOAMI" | jq -r '.id // empty' 2>/dev/null)
    CLAIM=$(printf '%s' "$WHOAMI" | jq -r '.claim // empty' 2>/dev/null)
    INTENT=$(printf '%s' "$WHOAMI" | jq -r '.intent // empty' 2>/dev/null)
else
    AGENT=$(printf '%s' "$WHOAMI" | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    CLAIM=$(printf '%s' "$WHOAMI" | sed -n 's/.*"claim"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    INTENT=$(printf '%s' "$WHOAMI" | sed -n 's/.*"intent"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
fi

[ -z "$AGENT" ] && exit 0

# Last few message lines on the active claim's thread (or global if none).
TAIL_THREAD="${CLAIM:-global}"
TAIL_OUT=$(_squad_run tail --since 2h --thread "$TAIL_THREAD" 2>/dev/null | tail -n 6)

LINES="claim: ${CLAIM:-(none)}"
[ -n "$INTENT" ] && LINES="$LINES
intent: $INTENT"
[ -n "$TAIL_OUT" ] && LINES="$LINES
recent on #$TAIL_THREAD:
$TAIL_OUT"

# Emit the PreCompact hookSpecificOutput envelope so the post-compact
# context retains the agent's identity and active state.
printf '%s' "$LINES" | (
    if command -v jq >/dev/null 2>&1; then
        jq -Rsc --arg agent "$AGENT" '{
            hookSpecificOutput: {
                hookEventName: "PreCompact",
                additionalContext: ("[squad identity survival]\nyou are " + $agent + "\n" + .)
            }
        }'
    else
        ESCAPED=$(sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | awk 'BEGIN{ORS="\\n"}{print}')
        printf '{"hookSpecificOutput":{"hookEventName":"PreCompact","additionalContext":"[squad identity survival]\\nyou are %s\\n%s"}}' "$AGENT" "$ESCAPED"
    fi
)
exit 0
