---
id: FEAT-031
title: inbox SPA replaces Refine row button with Auto-refine + drafting spinner + badge
type: feature
priority: P2
area: web-ui
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777308127
accepted_by: web
accepted_at: 1777308350
epic: auto-refine-inbox
references:
  - internal/server/web/inbox.js
  - internal/server/web/style.css
relates-to: []
blocked-by: [FEAT-030, FEAT-032]
---

## Problem

The inbox row in `internal/server/web/inbox.js` currently shows three buttons per item: Accept, Refine (manual comments composer), Reject. The Refine button on the row is the wrong tool for items captured with placeholder AC — it asks the human for refinement notes when the human just wants the body filled in. Replace the row Refine button with Auto-refine; the manual-comments flow remains available from the item-detail panel (TASK-042 verifies that).

## Context

`inbox.js:114` is the per-row action template. `openRefineComposer` at `inbox.js:230` is the manual-comments composer launched from that button. The new Auto-refine button POSTs to `/api/items/{id}/auto-refine` (FEAT-030), shows a "drafting…" spinner state during the call, and on 200 re-renders the row using the response payload (the server returns the updated item JSON). On error a toast fires using the existing `toast()` helper (`inbox.js` already calls it in the manual-refine error path at line ~273).

The "auto-refined" badge is a small label in the row (`<span class="auto-refined-badge">auto-refined</span>`) shown when `auto_refined_at` is set on the item. CSS lives next to `.refine-composer` in `internal/server/web/style.css`. Badge persists after acceptance — it is an audit marker, not a transient state.

## Acceptance criteria

- [ ] Per-row Refine button at `internal/server/web/inbox.js:114` is replaced with Auto-refine; manual-comments composer is no longer reachable from the inbox row (it remains reachable from the item-detail panel — verified by TASK-042).
- [ ] Clicking Auto-refine sets the button to a disabled "drafting…" state with a spinner; concurrent click on the same row is suppressed client-side; clicks on other rows continue to work.
- [ ] On 200 response the row re-renders from the response payload (no extra fetch); on error the button returns to its idle state and a toast fires with the server-supplied error message.
- [ ] Specific toast titles for the documented error codes: 503 ("Claude CLI not found"), 504 ("Auto-refine timed out"), 502 ("Claude failed: <stderr snippet>"), 409 ("Already drafting" or "Status is no longer captured" depending on body), 500 ("Claude exited without drafting").
- [ ] The inbox row renders an "auto-refined" badge when the item's JSON includes a non-zero `auto_refined_at` (provided by FEAT-032). Badge styling shares the existing warn-color palette used by the refine-composer accent.
- [ ] No confirmation modal — click fires immediately, even on bodies that are not template placeholders (per design decision).
- [ ] Re-clicking Auto-refine after a previous draft works the same as the first click; the badge `auto_refined_at` updates but the badge presence is unchanged.
- [ ] A unit test (or a Playwright-style smoke if the project has one — check before adding) covers the click-spinner-toast flow with a mocked `fetch`.

## Notes

The existing inbox SSE event `inbox_changed` already drives row re-renders when the server publishes one — so even if the auto-refine HTTP response were dropped on the floor, the SSE wired by FEAT-029's tool would still cause the row to refresh. The 200-response render is the fast path; the SSE is the safety net.

If the project has no JS test harness, ship this without a unit test and rely on TASK-041's Go integration test at the HTTP boundary plus a manual smoke note in the PR description. Do not add a JS test framework just for this item.
