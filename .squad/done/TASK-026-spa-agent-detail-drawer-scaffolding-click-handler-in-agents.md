---
id: TASK-026
title: SPA agent-detail drawer scaffolding + click handler in AGENTS panel
type: task
priority: P2
area: spa
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777251708
accepted_by: agent-bbf6
accepted_at: 1777251708
epic: agent-activity-stream
references:
  - internal/server/web/agents.js
  - internal/server/web/index.html
  - internal/server/web/style.css
relates-to:
  - TASK-025
blocked-by:
  - TASK-025
---

## Problem

The SPA's AGENTS panel renders agent rows but doesn't have a click-through. Tapping an agent should open a drawer (right-side or bottom panel — match the existing inbox-modal pattern) that shows the agent-detail timeline. This item builds the scaffolding only — the timeline rendering itself is TASK-027.

## Context

The existing inbox modal in `internal/server/web/inbox.js` is the pattern to follow — it opens on a click, renders details fetched from the server, closes on backdrop click or Escape. Read it end-to-end before implementing the drawer.

## Acceptance criteria

- [ ] `internal/server/web/agents.js` (or wherever the AGENTS panel renderer lives) gains a click handler on each agent row that opens a new agent-detail drawer.
- [ ] New drawer component (file or in-place; match the inbox modal's home) with:
  - Header: agent id, current claim if any, last-tick timestamp
  - Content area: empty placeholder for now (rendered in TASK-027)
  - Close button + backdrop click + Escape key all close the drawer
- [ ] Drawer fetches `GET /api/agents/:id/events?limit=50` on open (so the network round-trip is in flight before TASK-027's renderer lands; renderer will read the cached response). Shows a loading spinner while pending.
- [ ] CSS additions in `style.css` (or a new `agents-detail.css`) for the drawer layout. Match the existing color tokens / spacing scale.
- [ ] No tests required (SPA is JS, not Go) — but confirm by manual smoke: open the SPA via `squad serve`, click an agent, drawer opens, network panel shows the events fetch fired. Document the smoke-test steps in the close-out attestation.
- [ ] No regressions in the AGENTS panel — clicking still selects-or-whatever the existing UX is, and the drawer doesn't break the underlying click target.

## Notes

- Drawer placement: side-drawer (right side, ~40% viewport width on desktop, full-width on mobile) or bottom-drawer — pick whichever fits the existing SPA aesthetic. Don't bikeshed.
- The fetched events response is cached on the drawer instance for TASK-027 to read. No state-management library introduced.
- Keep the drawer scaffolding self-contained — TASK-027 will add the timeline list, TASK-028 will add SSE updates. Don't pre-build affordances for those; just leave hooks (e.g. a `renderTimeline(events)` callback the drawer calls when events arrive).

## Resolution

(Filled in when status → done.)
