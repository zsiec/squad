---
id: BUG-018
title: inbox modal rows go stale on inbox_changed SSE — only the badge updates
type: bug
priority: P3
area: web-ui
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-27"
captured_by: agent-1f3f
captured_at: 1777246939
accepted_by: web
accepted_at: 1777247092
references:
  - internal/server/web/inbox.js
  - internal/server/web/app.js
relates-to: []
blocked-by: []
---

## Problem

If the inbox modal is open when an `inbox_changed` SSE event arrives (because another reviewer accepted/rejected/refined an item, or a refining agent recaptured one), the open modal's row list does not re-render. The badge count updates, but the rows are stale — clicking accept/reject/refine on a row that no longer exists 404s.

## Context

`internal/server/web/app.js` listens for `inbox_changed` and only calls `refreshInboxCount()` (the badge-fetch helper). The modal's row list lives in `internal/server/web/inbox.js`'s `renderList()` which is private to that module. There's no exported "re-render if open" function.

Pre-existing hole — present before the inbox-refinement epic. Surfaced during the epic's final integration review because the new `refine`/`recapture` actions widen the surface area where a stale modal misleads the reviewer.

## Acceptance criteria

- [ ] When an `inbox_changed` SSE arrives and the inbox modal is open, the row list re-renders to reflect the current `/api/inbox` payload.
- [ ] Badge count still updates as before.
- [ ] Closed-modal behavior unchanged (no extra fetches when nobody's looking).
- [ ] One regression test (manual smoke if no JS test infra; otherwise an integration test that opens the modal and posts a refine, asserting the row disappears).

## Notes

Smallest fix: export a `renderListIfOpen()` (or `refreshOpenModal()`) from `inbox.js`; call it from the `inbox_changed` SSE handler in `app.js`. Avoid double-fetching: the existing `refreshInboxCount()` already hits `/api/inbox` to count; the modal re-render also hits it for the row data. Could share one fetch. Optional optimization.

Discovered during inbox-refinement epic final review.

## Resolution

### Fix

`internal/server/web/inbox.js` — exported a new `refreshIfOpen()` that re-renders the modal's row list when `modalEl` is visible, no-op when closed. Closed-modal sessions still pay zero extra fetch cost.

`internal/server/web/app.js` — `inbox_changed` SSE handler now calls both `refreshInboxCount()` (badge) AND `refreshInboxIfOpen()` (modal rows). When the modal is closed, the second call early-returns without fetching.

### Reproduction / evidence

Playwright (`/tmp/browser-test/inbox-sse.js`):

```
=== open inbox modal ===
inbox rows visible: [ 'BUG-019' ]
=== POST /refine for BUG-019 (triggers inbox_changed SSE) ===
refine status: 204
inbox rows after refine: []
OK
```

A captured `BUG-019` was visible in the open modal; after a `POST /refine` (which moves it to needs-refinement and emits `inbox_changed`), the modal auto-re-rendered with no rows — the SSE-driven refresh now reflects current state.

Pre-fix: the modal would still show the stale `BUG-019` row indefinitely; clicking accept/reject on it would 404.

### AC verification

- [x] Inbox modal re-renders on `inbox_changed` when open.
- [x] Badge count updates as before (refreshInboxCount path unchanged).
- [x] Closed-modal: `refreshIfOpen` early-returns without fetching — no extra network.
- [x] Manual smoke captured via Playwright; full `go test ./... -count=1 -race` green.
