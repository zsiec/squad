---
id: TASK-008
title: SPA — Refine button + inline composer in inbox modal
type: task
priority: P2
area: web-ui
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777242007
accepted_by: agent-1f3f
accepted_at: 1777242007
epic: inbox-refinement
references:
  - internal/server/web/inbox.js
relates-to:
  - TASK-009
blocked-by:
  - TASK-003
---

## Problem

The inbox modal needs a third action between Accept and Reject: "Refine" (warn-tier). Clicking it auto-expands the detail panel and reveals an inline textarea + Send/Cancel buttons. Submitting POSTs to `/api/items/{id}/refine`.

## Context

Modify `internal/server/web/inbox.js`:

1. Add a `Refine` button to the row template's `inbox-actions`.
2. Wire it through the existing `onClick` dispatcher (new `data-action="refine"` branch).
3. Implement `openRefineComposer(id)` that auto-expands details (calls `toggleDetails` if hidden) and injects a `.refine-composer` div with textarea + Send/Cancel buttons.
4. Send disabled while textarea is empty; on click, POST to `/api/items/{id}/refine` with `{comments}`, toast, re-render list.

Full reference (with paste-in code) in Task 8 of the implementation plan.

## Acceptance criteria

- [x] `node --check internal/server/web/inbox.js` passes.
- [x] `go build ./...` passes (no Go changes, but bundles assets).
- [x] Manual smoke (in TASK-010): clicking Refine on a captured item auto-expands details, focuses textarea, Send is disabled until non-empty, sending shows a "Sent X for refinement" toast and the row disappears.
- [x] Existing Accept / Reject / Details paths unchanged.

## Notes

Depends on TASK-003 (the endpoint must exist). CSS lives in TASK-009.
