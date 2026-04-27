#!/usr/bin/env bash
# Skill-scoped: only fires while squad-loop is the active skill.
# Cheaper than user-prompt-tick because it runs only at Bash boundaries
# inside actively-coordinating sessions, not at every prompt.
set -eu
[ -n "${SQUAD_NO_HOOKS:-}" ] && exit 0
if ! command -v squad >/dev/null 2>&1; then
  exit 0
fi
output=$(squad tick 2>/dev/null || true)
if [ -n "$output" ]; then
  if command -v jq >/dev/null 2>&1; then
    encoded=$(printf '%s' "$output" | jq -Rs .)
  else
    # Manual JSON-string escape: backslash, double-quote, and the control
    # characters JSON disallows in a string. awk handles the line-by-line
    # join with literal "\n" so multi-line tick output survives the trip.
    encoded=$(printf '%s' "$output" \
      | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' \
            -e 's/\r/\\r/g' -e 's/\t/\\t/g' \
      | awk 'BEGIN{ORS=""} NR>1{print "\\n"} {print}')
    encoded="\"${encoded}\""
  fi
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":%s}}\n' "$encoded"
fi
exit 0
