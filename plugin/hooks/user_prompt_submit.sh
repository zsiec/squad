#!/bin/sh
# squad UserPromptSubmit hook — auto-tick before Claude reads the prompt.
# When mentions, knocks, handoffs, or lost claims are pending, inject them
# as additionalContext so Claude sees them on this turn instead of waiting
# for the user to remember `squad tick`. SQUAD_NO_HOOKS=1 disables.

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

DIGEST=$(_squad_run tick --json 2>/dev/null)
if [ -z "$DIGEST" ]; then
    exit 0
fi

# Use jq when available — clean, handles every digest section. Fall back
# to a literal "anything non-empty?" check otherwise; emit the raw digest
# rather than synthesising a polished summary so squad still helps even
# without jq installed.
if command -v jq >/dev/null 2>&1; then
    HAS_NEWS=$(printf '%s' "$DIGEST" | jq -r '
        if (.knocks // [] | length) > 0
            or (.mentions // [] | length) > 0
            or (.handoffs // [] | length) > 0
            or (.your_threads // [] | length) > 0
            or (.lost_claims // [] | length) > 0
        then "yes" else "" end' 2>/dev/null)
    if [ -z "$HAS_NEWS" ]; then
        exit 0
    fi
    SUMMARY=$(printf '%s' "$DIGEST" | jq -r '
        def section(name; arr):
            if (arr // [] | length) == 0 then empty
            else "\(name):\n" + (arr | map("  - [\(.kind // "say")] @\(.agent // "?") on #\(.thread // "?"): \(.body // "")") | join("\n"))
            end;
        [
            (if (.lost_claims // [] | length) > 0 then "RECLAIMED while away: " + (.lost_claims | join(", ")) else empty end),
            section("KNOCKS (high priority)"; .knocks),
            section("MENTIONS"; .mentions),
            section("YOUR THREADS"; .your_threads),
            section("HANDOFFS"; .handoffs)
        ] | map(select(. != null)) | join("\n\n")
    ' 2>/dev/null)
else
    # No jq — best-effort detection. If the digest contains anything other
    # than empty arrays, surface it raw.
    case "$DIGEST" in
        *'"mentions": ['*']'*'"knocks": ['*']'*) exit 0 ;;
    esac
    SUMMARY="squad inbox (raw, install jq for a tidy version): $DIGEST"
fi

if [ -z "$SUMMARY" ]; then
    exit 0
fi

# Claude Code's UserPromptSubmit hook accepts JSON output of the form
# {"hookSpecificOutput":{"hookEventName":"UserPromptSubmit",
#                        "additionalContext":"..."}} per
# https://code.claude.com/docs/en/hooks. The additionalContext string is
# injected into the model's context for this turn.
printf '%s' "$SUMMARY" | (
    if command -v jq >/dev/null 2>&1; then
        jq -Rsc '{
            hookSpecificOutput: {
                hookEventName: "UserPromptSubmit",
                additionalContext: ("[squad inbox]\n" + .)
            }
        }'
    else
        # Cheap escaper for the JSON string body. Newlines + double quotes
        # + backslashes are the only chars that matter in additionalContext.
        ESCAPED=$(sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | awk 'BEGIN{ORS="\\n"}{print}')
        printf '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"[squad inbox]\\n%s"}}' "$ESCAPED"
    fi
)
exit 0
