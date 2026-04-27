---
id: FEAT-052
title: SPA button to send needs-refinement item back to captured
type: feature
priority: P2
area: web
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777328560
accepted_by: web
accepted_at: 1777328644
references: []
relates-to: []
blocked-by: []
---

## Problem
Items bounced from a reviewer with `## Reviewer feedback` sit in `needs-refinement` until someone claims and runs `squad recapture` from a CLI. The SPA inbox displays these items but offers no UI affordance to send them back to `captured` — the user has to drop to a terminal even when the reviewer's note is no longer relevant (e.g. a DoR rule was removed). Adds friction for a common-enough action.

## Context
Backend already has the pieces: `POST /api/items/{id}/claim` (`internal/server/server.go:124`) and `POST /api/items/{id}/recapture` (`server.go:122`, handler at `internal/server/items_recapture.go`). The recapture handler requires `X-Squad-Agent` header and that the caller hold the claim. The SPA can chain claim-then-recapture under the existing `web` agent identity (matching how `Auto-refine` already impersonates the user).

## Acceptance criteria
- [ ] SPA renders a "Send back to captured" button on items with status `needs-refinement` (item-detail page is sufficient for v1; inbox row is bonus).
- [ ] Click sequentially calls POST /api/items/{id}/claim then POST /api/items/{id}/recapture under agent `web`.
- [ ] Success path: status flips to `captured`, the inbox SSE stream emits `inbox_changed action=recapture`, the row re-renders without a manual refresh.
- [ ] Error paths surface a clear message: 409 (item held by another agent) shows "claim held by X", non-2xx shows the server error body.
- [ ] No regression in existing recapture handler tests.

## Notes
v1 punts on a confirm dialog — the user is explicitly choosing to override the reviewer's note, no extra friction needed. v2 could surface the reviewer-feedback text inline as context. Out of scope: editing the body before recapture (that's the full refinement flow, separate UI).

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
