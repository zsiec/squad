#!/usr/bin/env bash
# Skill-scoped: only fires while squad-loop is the active skill.
# Cheaper than user-prompt-tick because it runs only at Bash boundaries
# inside actively-coordinating sessions, not at every prompt.
set -euo pipefail
[ -n "${SQUAD_NO_HOOKS:-}" ] && exit 0
if ! command -v squad >/dev/null 2>&1; then
  exit 0
fi
output=$(squad tick 2>/dev/null || true)
if [ -n "$output" ]; then
  printf '{"hookSpecificOutput":{"additionalContext":%s}}\n' \
    "$(printf '%s' "$output" | jq -Rs .)"
fi
exit 0
