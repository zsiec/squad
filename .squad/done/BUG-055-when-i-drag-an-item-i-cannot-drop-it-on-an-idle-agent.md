---
id: BUG-055
title: When I drag an item, I cannot drop it on an idle agent.
type: bug
priority: P1
area: web
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: web
captured_at: 1777350278
accepted_by: web
accepted_at: 1777350727
references: []
relates-to: []
blocked-by: []
auto_refined_at: 1777350716
auto_refined_by: claude
---

## Problem
On the dashboard SPA, dragging a ready item out of the items table fails to drop on an idle agent in the Agents column. The drag initiates (row gets `dragging`, body gets `dnd-item-active`) but releasing over an idle agent row produces no `assignItemToAgent` call, no `POST /api/items/<id>/assign`, and the item stays unassigned. The expected behaviour is that an idle agent row accepts the drop and the existing `Assigned <ID> → <agent name>` toast appears.

## Context
Drag source: `internal/server/web/board.js:206` (`tr.draggable = true` plus `dragstart` setting both `text/plain` and `application/x-squad-item`).

Drop wiring: `wireIdleDrop` at `internal/server/web/board.js:356`. It registers `dragenter`/`dragover`/`dragleave`/`drop` only when the rendered agent's state is `idle` (gated at line 345). The drop handler reads the item id from `application/x-squad-item` (with `text/plain` fallback) and calls `assignItemToAgent`, which posts to `/api/items/:id/assign`.

Reproducer: open the dashboard, switch to the Ready tab so a ready item row is draggable, drag it onto any agent row whose state dot reads `idle`, release. Observe no toast and no `/assign` network call.

Suspected root causes (each must be empirically checked against current code before any fix lands):

1. `dragenter` only calls `preventDefault()` inside the `isItemDrag(e)` guard at line 358; without an unconditional prevent on the very first enter, the browser may treat the row as a non-target and suppress subsequent `dragover` events.
2. The `dragleave` handler at line 367 strips `data-drop-target` on every bubble out of a child (`.agent-body` and descendants), so the affordance flickers and the user releases at a moment when the `<li>` is not styled as a valid drop zone.
3. `wireIdleDrop` is wired only when the agent renders as `idle`; if SSE pushes an `active → idle` status update without re-running `renderAgents()`, the listeners are absent on the row the user sees.

## Acceptance criteria
- [ ] Each of the three suspected root causes above is either confirmed and fixed in `internal/server/web/board.js`, or empirically ruled out with a one-line note in the commit body — no fix lands without first verifying which of (1)/(2)/(3) actually reproduces the failure.
- [ ] Manual reproduction in a running dashboard: dragging a ready row from the Ready tab onto an idle agent row produces a `POST /api/items/<ID>/assign` request (visible in DevTools Network) with `agent_id` matching the drop target, and the existing `Assigned <ID> → <agent name>` toast appears. Evidence pasted into the item resolution notes: the request line and the toast text copied from DevTools.
- [ ] Manual verification confirms the `data-drop-target` attribute on the agent `<li>` stays set for the entire duration the cursor is anywhere inside that `<li>`, including while hovering over `.agent-body` and other descendants — checked by watching the attribute in DevTools' Elements panel while moving the cursor across child elements.
- [ ] Manual reproduction of the negative case: dragging a ready row onto an agent whose state dot reads `active` or `stopped` produces zero `/assign` requests and no toast. Evidence pasted into the resolution notes: the DevTools Network log (or a copy of its filtered rows) showing no `/assign` call for the negative attempt.
