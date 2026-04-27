---
id: FEAT-051
title: wire timeBoxNudge into squad listen pollMailbox for true async wakeup
type: feature
priority: P2
area: cmd/squad
status: captured
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: 2026-04-27
captured_by: agent-afcd
captured_at: 1777321662
accepted_by: ""
accepted_at: 0
references: []
relates-to: []
blocked-by: []
---

## Problem

FEAT-042 landed the 2h time-box wakeup as a tick-driven nudge — fires
on the next `squad tick` (PreToolUse Bash boundary) after the
threshold. If the agent goes silent for 2h with no Bash boundary, the
nudge never fires; the exact failure mode the time-box rule was meant
to prevent.

## Context

`cmd/squad/listen.go` + `plugin/hooks/async_rewake.sh` already provide
a truly-async long-poll: bind a TCP listener, poll the mailbox on a
30s fallback, exit code 2 with a stderr reminder body that Claude Code
injects mid-turn.

The fix is to extend the listener's poll to ALSO surface time-due
nudges: when the held claim has crossed an unfired 90m/120m threshold,
the listener returns the same `maybePrintTimeBoxNudge` text as the
body of an exit-2 wake. The marker-on-claim dedupe applies, so the
listener and the tick path can both fire safely without double-emit.

`cmd/squad/cadence_nudge.go` already exposes `timeBoxNudgeText` /
`maybePrintTimeBoxNudge` / `lastMilestoneSilence` — the listener
integration is plumbing, not new logic.

## Acceptance criteria

- [ ] The listener's poll surfaces a time-box nudge body when the held
      claim has crossed an unfired threshold; the existing
      async_rewake.sh path emits it as a system message without
      waiting for a Bash boundary.
- [ ] Marker-on-claim dedupe prevents the listener and the tick path
      from double-firing.
- [ ] Test exercises: mailbox empty + threshold crossed → wake with
      time-box body; mailbox empty + threshold not crossed → fallback
      timeout (no body); marker stamped by listener path → tick path
      is a no-op.
- [ ] Existing async_rewake hook contract (exit 2 + stderr) is
      preserved.

## Notes

FEAT-042 reviewer flagged this as the "true async" half of FEAT-042's
AC. Captured here so the time-boxing skill's 2h cap actually catches
silent agents, not just talkative ones. Phase-1 tick-driven nudge
already shipped; this is phase 2.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
