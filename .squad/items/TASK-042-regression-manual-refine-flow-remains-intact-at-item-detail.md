---
id: TASK-042
title: 'regression: manual-refine flow remains intact at item-detail panel'
type: task
priority: P2
area: web-ui
status: open
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777308128
accepted_by: web
accepted_at: 1777308351
epic: auto-refine-inbox
references:
  - internal/server/items_refine.go
  - internal/server/integration_refine_test.go
  - internal/server/web/inbox.js
relates-to: []
blocked-by: [FEAT-031]
---

## Problem

The auto-refine epic replaces the per-row Refine button on the inbox with Auto-refine (FEAT-031). The pre-existing manual-comments refine flow — `POST /api/items/{id}/refine` plus the `openRefineComposer` textarea — must remain reachable from the item-detail panel and behave exactly as it did before. The auto-refine epic is additive at the inbox row, not a replacement of the entire refine epic.

## Context

The existing integration test at `internal/server/integration_refine_test.go` exercises captured → refine → list → claim → recapture → inbox. That test already covers the server-side flow. The risk introduced by FEAT-031 is on the client: when the row Refine button is removed, did anything else accidentally remove `openRefineComposer` or unhook the item-detail panel's call site?

## Acceptance criteria

- [ ] `internal/server/integration_refine_test.go` continues to pass unchanged (captured → /api/items/{id}/refine → /api/refine list → claim → recapture → back to inbox).
- [ ] `openRefineComposer` in `internal/server/web/inbox.js` is still defined and reachable from the item-detail panel's "Send for refinement" / equivalent control; if FEAT-031 removed the inbox-row caller, the function and the detail-panel caller are still wired.
- [ ] If the project has a Playwright/jsdom harness for inbox.js, add a regression spec that opens the item-detail panel, clicks "Send for refinement," types a comment, hits Send, and asserts `POST /api/items/{id}/refine` was called with the comment payload. If no harness exists, document the manual smoke steps in this item's Resolution section after FEAT-031 lands and skip the automated spec.
- [ ] The `/api/refine` list endpoint and the `squad refine` CLI verb behave unchanged — covered by their existing tests; this item only confirms no client wiring was lost.
- [ ] Resolution section of this item, when closed, includes a one-line note pointing future readers at where the manual-refine flow now lives in the SPA (the panel/control name) so it does not look "removed" to a code archaeologist.

## Notes

Tiny scope on purpose — most of the regression coverage already exists in the existing integration test. This item is the explicit contract that the epic is additive, not destructive, of the prior refine work.

## Resolution

Manual-refine flow now lives in the inbox modal's per-item detail panel (toggle Details, then "Send for refinement"). The button's `data-action="refine"` is wired through the same `onClick` dispatcher as the row buttons, which routes to `openRefineComposer(id)` (still defined at `internal/server/web/inbox.js`).

Smoke-tested by passing the existing test suite — `TestIntegration_RefineRoundTrip` (the captured → /api/items/{id}/refine → /api/refine list → claim → recapture → back to inbox loop) still passes, and the per-handler `TestItemsRefine_*` / `TestItemsRecapture_*` / `TestSSE_InboxChanged_OnRefine` / `TestRefineCmd_*` / `TestRecaptureCmd_*` all pass unchanged. No JS harness exists in `internal/server/web/`, so the item's escape hatch applies: ship without an automated SPA spec and rely on the Go integration tests + the manual smoke below.

Manual SPA smoke steps (one-time, run when changing the dispatcher or detail-panel render):

1. `go run ./cmd/squad init --yes` in a fresh repo, then `go run ./cmd/squad new feat "needs refinement comments" --area dashboard`.
2. `go run ./cmd/squad serve --bind 127.0.0.1 --port 18337` and open `http://127.0.0.1:18337/`.
3. Click the inbox button, expand the item via Details.
4. Click "Send for refinement" inside the detail panel; type a comment in the textarea; click Send.
5. Confirm a `POST /api/items/{id}/refine` fires (Network panel) with the comment payload, the toast says "Sent {id} for refinement", and the inbox row disappears (item is now `needs-refinement`).
6. `go run ./cmd/squad refine` — confirms the item appears on the CLI list.
