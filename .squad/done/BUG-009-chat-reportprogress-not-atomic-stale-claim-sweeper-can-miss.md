---
id: BUG-009
title: chat ReportProgress not atomic — stale-claim sweeper can miss claims
type: bug
priority: P1
area: chat
status: done
estimate: 30m
risk: medium
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241456
accepted_by: web
accepted_at: 1777241672
references: []
relates-to: []
blocked-by: []
---

## Problem

`internal/chat/progress.go:16-31` performs three independent `ExecContext` calls — INSERT into `progress`, UPDATE `agents.last_tick_at`, UPDATE `claims.last_touch` — without wrapping them in a transaction. If the context cancels, the DB is briefly busy, or the process dies between calls, the writes commit partially. The most damaging interleaving: the progress row is recorded but `claims.last_touch` is never updated; a concurrent stale-claim sweep then sees `last_touch` past the threshold and treats the claim as abandoned, even though the agent is actively reporting.

## Context

The hygiene sweeper (`internal/hygiene/`) decides "is this claim alive?" by reading `claims.last_touch`. `ReportProgress` is one of the primary refresh paths for that timestamp — it is called by the user-prompt-tick hook on every turn the agent takes. A partial write here is silent and looks identical to a real abandonment, which would cause the sweeper to release the claim out from under a working agent.

## Acceptance criteria

- [ ] Three writes wrapped in `store.WithTxRetry` (or equivalent) so they commit atomically or not at all.
- [ ] `c.bus.Publish` moves outside the transaction — only fire on successful commit (publish-after-commit).
- [ ] Test: simulate context cancellation between INSERT and the second UPDATE; assert that either all three writes happened or none did.
- [ ] No regression in existing `ReportProgress` callers.

## Notes

Sibling pattern to BUG-010 (claims/release.go). Both are "multi-statement write that should be one transaction". Found during a parallel exploration sweep on 2026-04-26 and verified by reading the file directly.

## Resolution
(Filled in when status → done.)
