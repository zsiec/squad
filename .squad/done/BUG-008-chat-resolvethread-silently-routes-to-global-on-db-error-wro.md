---
id: BUG-008
title: chat ResolveThread silently routes to global on DB error — wrong-thread messages during outage
type: bug
priority: P1
area: chat
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241453
accepted_by: web
accepted_at: 1777241668
references: []
relates-to: []
blocked-by: []
---

## Problem

`internal/chat/route.go:12-14` swallows the `QueryRowContext`/`Scan` error with `_ =`. If the DB is briefly unreachable (or the row scan fails for any reason), `item` stays empty and the function silently falls through to `ThreadGlobal`. The caller has no signal that the lookup failed — chatty verbs that should have been routed to the agent's current claim end up in the global thread instead.

## Context

`ResolveThread` is on the hot path for every typed chat verb (`squad thinking`, `milestone`, `stuck`, `fyi`, `ask`) when no `--thread` override is passed. Lost routing means the audit trail for an item gets gaps and peers monitoring that item's thread miss the update. The bug is a classic swallowed-error pattern — `errors.Is(err, sql.ErrNoRows)` is the only condition that should silently fall through to global; everything else should propagate.

## Acceptance criteria

- [ ] `ResolveThread` returns an `(string, error)` pair (or logs the unexpected error path), so a real DB failure is no longer indistinguishable from "agent has no current claim".
- [ ] `sql.ErrNoRows` continues to map to `ThreadGlobal` without producing an error — that's the documented "no current claim" path.
- [ ] All call sites updated to handle the error (or log it). No new silent `_ =` introduced.
- [ ] Test added: with a closed/broken DB, `ResolveThread` does NOT silently return `ThreadGlobal`.

## Notes

Found during a parallel exploration sweep on 2026-04-26. Verified by reading the file directly. Related anti-pattern lives in `internal/chat/post.go:63` (`id, _ = res.LastInsertId()`) — that one tracked separately under CHORE-002.

## Resolution
(Filled in when status → done.)
