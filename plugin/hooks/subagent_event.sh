#!/bin/sh
# squad subagent/task hook — record SubagentStart/Stop and TaskCreated/Completed
# events into the squad store and bump the parent agent's heartbeat. Default ON.
# SQUAD_NO_HOOKS=1 disables. Always exits 0; never blocks Claude.

set -u

if [ "${SQUAD_NO_HOOKS:-0}" = "1" ]; then
    exit 0
fi

SQUAD_BIN="${SQUAD_BIN:-squad}"
if ! command -v "$SQUAD_BIN" >/dev/null 2>&1; then
    exit 0
fi

if command -v gtimeout >/dev/null 2>&1; then
    gtimeout 2s "$SQUAD_BIN" subagent-event >/dev/null 2>&1
elif command -v timeout >/dev/null 2>&1; then
    timeout 2s "$SQUAD_BIN" subagent-event >/dev/null 2>&1
else
    "$SQUAD_BIN" subagent-event >/dev/null 2>&1
fi
exit 0
