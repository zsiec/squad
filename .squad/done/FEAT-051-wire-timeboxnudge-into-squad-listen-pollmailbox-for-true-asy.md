---
id: FEAT-051
title: wire timeBoxNudge into squad listen pollMailbox for true async wakeup
type: feature
priority: P2
area: cmd/squad
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777321662
accepted_by: web
accepted_at: 1777328570
references: []
relates-to: []
blocked-by: []
auto_refined_at: 1777328556
auto_refined_by: claude
---
## Problem

The 2h time-box wakeup from FEAT-042 fires on the next `squad tick`
(PreToolUse Bash boundary) after the threshold crosses. If the agent
goes silent for two hours with no Bash boundary, the nudge never fires —
the exact failure mode the time-box rule was meant to prevent.

## Context

`cmd/squad/listen.go` and `plugin/hooks/async_rewake.sh` already implement
a truly-async long-poll: bind a TCP listener, poll the mailbox on a 30s
fallback, exit code 2 with a stderr reminder body that Claude Code injects
mid-turn.

`cmd/squad/cadence_nudge.go` exposes `timeBoxNudgeText`,
`maybePrintTimeBoxNudge`, and `lastMilestoneSilence` — the listener
integration is plumbing, not new logic.

The marker-on-claim dedupe stamped on first emit lets the listener and
the tick path both attempt to fire without double-emitting.

## Acceptance criteria

- [ ] `squad listen`'s mailbox poll returns the `timeBoxNudgeText` body via exit code 2 on stderr when the held claim has crossed an unfired 90m or 120m threshold, without waiting for a `squad tick` boundary.
- [ ] When the listener emits a time-box nudge it stamps the dedupe marker so a subsequent `squad tick` against the same threshold is a no-op.
- [ ] A test in `cmd/squad/` covers three cases: mailbox empty plus threshold crossed yields exit 2 with the time-box body; mailbox empty plus threshold not crossed yields the 30s fallback timeout with no body; marker stamped by the listener path makes the tick path emit nothing.
- [ ] The existing `async_rewake.sh` contract (exit code 2 with the body on stderr, no stdout) is preserved unchanged.

## Notes

FEAT-042 reviewer flagged this as the "true async" half of FEAT-042's AC.
Phase-1 tick-driven nudge already shipped; this is phase 2.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
