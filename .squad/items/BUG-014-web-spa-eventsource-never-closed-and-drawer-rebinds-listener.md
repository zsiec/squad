---
id: BUG-014
title: web SPA — EventSource never closed and drawer rebinds listeners on each open
type: bug
priority: P2
area: spa
status: open
estimate: 45m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241475
accepted_by: web
accepted_at: 1777241684
references: []
relates-to: []
blocked-by: []
---

## Problem

Two related leaks in the dashboard SPA:

1. `internal/server/web/app.js:196` opens an `EventSource` and never calls `.close()`. The connection persists across hidden tabs and stays attached to a never-released closure.
2. `internal/server/web/drawer.js:159-171` re-runs `querySelectorAll('.dep-chip').forEach(c => c.addEventListener('click', …))` every time a drawer opens. The previous DOM is replaced via `innerHTML`, but listeners attached to the *new* nodes accumulate references each time `renderMarkdown` rebuilds the body, and any held references to the old DOM nodes leak.

## Context

A long-lived dashboard tab is the expected mode of operation — peers leave the dashboard open all day. Resource accumulation is not academic; over a long session it shows up as growing memory and stuck handlers. Both fixes are small.

## Acceptance criteria

- [ ] `app.js` closes the EventSource on `beforeunload` (and ideally pauses it on `visibilitychange === "hidden"` to drop bandwidth on background tabs).
- [ ] `drawer.js` either uses event delegation on a stable parent OR removes prior listeners before re-binding.
- [ ] Verified manually: open the dashboard, repeatedly open/close the same drawer, confirm in DevTools that listener count stays flat instead of growing.

## Notes

Found during a parallel exploration sweep on 2026-04-26. Related: BUG-015 (SSE auth-failure handling) — same `app.js` SSE wiring.

## Resolution
(Filled in when status → done.)
