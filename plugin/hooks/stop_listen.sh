#!/bin/sh
# squad Stop hook — listen-and-block. Default ON when squad install detects
# loopback bind works. Bound to a 24h Claude Code hook timeout. Exits 2
# with {"decision":"block","reason":"<inbox>"} when a peer message arrives;
# exits 0 silently if the timeout elapses idle. SQUAD_NO_HOOKS=1 disables.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

# Derive a stable instance id from the same envvars session_start.sh uses,
# so listen registers under the same key the SessionEnd cleanup will scrub.
SESSION_KEY="${SQUAD_SESSION_ID:-${TERM_SESSION_ID:-${ITERM_SESSION_ID:-${TMUX_PANE:-${WT_SESSION:-}}}}}"
if [ -n "$SESSION_KEY" ]; then
    SUFFIX=$(printf '%s' "$SESSION_KEY" | shasum 2>/dev/null | cut -c1-12)
    [ -z "$SUFFIX" ] && SUFFIX=$(printf '%s' "$SESSION_KEY" | cut -c1-12)
else
    SUFFIX=$(openssl rand -hex 6 2>/dev/null || printf '%s' "$$" | cut -c1-12)
fi
INSTANCE="session-$SUFFIX"

# `squad listen` exits 2 with decision-block JSON on wake; exits 0 silent on
# the 24h max-lifetime. We pass through stdout verbatim (Claude Code reads
# the JSON envelope from listen's stdout) and propagate the exit code so
# Claude Code's Stop-hook block-on-2 contract fires.
exec "$SQUAD_BIN" listen --instance "$INSTANCE" --max 24h --fallback 30s
