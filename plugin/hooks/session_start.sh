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

# Derive a stable per-session suffix so re-running the hook in the same
# Claude Code session resolves to the SAME agent_id (no DB pollution).
SESSION_KEY="${SQUAD_SESSION_ID:-${TERM_SESSION_ID:-${ITERM_SESSION_ID:-${TMUX_PANE:-${WT_SESSION:-}}}}}"
if [ -n "$SESSION_KEY" ]; then
    SUFFIX=$(printf '%s' "$SESSION_KEY" | shasum 2>/dev/null | cut -c1-4)
    [ -z "$SUFFIX" ] && SUFFIX=$(printf '%s' "$SESSION_KEY" | cut -c1-4)
else
    SUFFIX=$(openssl rand -hex 2 2>/dev/null || printf '%s' "$$" | cut -c1-4)
fi
AGENT_ID="agent-$SUFFIX"
AGENT_NAME="claude-$AGENT_ID"

"$SQUAD_BIN" register --as "$AGENT_ID" --name "$AGENT_NAME" --no-repo-check >/dev/null 2>&1 || true
"$SQUAD_BIN" tick >/dev/null 2>&1 || true

WHOAMI_ID=$("$SQUAD_BIN" whoami --json 2>/dev/null \
    | sed -n 's/.*"id":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
REPO_NAME=$("$SQUAD_BIN" workspace status --json 2>/dev/null \
    | sed -n 's/.*"current_repo":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
NEXT_TOP=$("$SQUAD_BIN" next --json --limit 3 2>/dev/null \
    | grep -oE '"id":[[:space:]]*"[^"]+"' \
    | sed -E 's/"id":[[:space:]]*"([^"]+)"/\1/' \
    | head -n3 | tr '\n' ',' | sed 's/,$//')

[ -z "$WHOAMI_ID" ] && exit 0
[ -z "$NEXT_TOP" ] && NEXT_TOP="(none)"
[ -z "$REPO_NAME" ] && REPO_NAME="(no repo)"

printf '[squad] registered as %s in %s; ready stack top: %s.\n' \
    "$WHOAMI_ID" "$REPO_NAME" "$NEXT_TOP"
exit 0
