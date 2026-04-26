---
id: BUG-006
title: web UI inbox shows captured items with approve/reject but no way to view item details first
type: bug
priority: P2
area: web-ui
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777239636
accepted_by: agent-1f3f
accepted_at: 1777239636
references: []
relates-to: []
blocked-by: []
---

## Problem

The web UI inbox surfaces captured items with approve / reject controls, but there's no way to view the item's full details (problem statement, context, acceptance criteria, etc.) before deciding. The reviewer is forced to approve or reject blind, or open a separate terminal to read the file.

## Context

The inbox modal lives in `internal/server/web/inbox.js`. Captured items have full markdown bodies (problem, context, AC) at `.squad/items/<ID>-*.md` — that data is already being served by the items API used elsewhere in the SPA (e.g. the drawer in `drawer.js`).

Likely fix shape: clicking the row, or a "details" affordance on each inbox row, should open the same drawer (`drawer.js`) the rest of the SPA uses for items. The drawer already renders markdown via `markdown.js`, so the body should display correctly without new rendering code.

## Acceptance criteria

- [x] From the inbox modal, the user can open the full details of a captured item (problem, context, AC, metadata) without first approving or rejecting it.
- [x] Approve / reject controls remain usable from the same surface; the user can decide after reading.
- [x] Existing inbox accept/reject behavior is unchanged for users who don't open details.
- [x] No regression in the rest of the SPA's item-drawer behavior (clicking an item in the board still works as before).

## Notes

Related to BUG-004 (broader web UI gaps around viewing items). Filing separately because the failure mode is different — BUG-004 is about closed/done items not being reachable; this one is about captured/inbox items not being inspectable before approval.

## Resolution

Added a `Details` toggle to each row in the inbox modal (`internal/server/web/inbox.js`). Clicking it lazy-fetches `/api/items/{id}` and renders metadata (type, priority, area, estimate, risk, created, updated), the AC checklist, and the body markdown inline below the row. The toggle re-collapses and re-uses the loaded payload on subsequent opens. Accept/Reject buttons remain on the same row.

Styling for the inline panel added to `internal/server/web/style.css` under `.inbox-details`.

Drawer (`drawer.js`) and the board click path are untouched — no regression there.

Pre-existing XSS in `markdown.js` (`javascript:` URLs slip past `escapeHtml` because `[`/`]`/`(`/`)`/`:` are not escape targets) is now exposed by this surface in addition to the drawer. Filed as BUG-007.
