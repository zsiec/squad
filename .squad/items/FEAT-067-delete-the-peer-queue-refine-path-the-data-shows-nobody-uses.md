---
id: FEAT-067
title: delete the peer-queue refine path the data shows nobody uses
type: feature
priority: P2
area: server
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352039
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
---

## Problem

The peer-queue refine flow (`POST /api/items/:id/refine`,
`handleItemsRefine` at `internal/server/items_refine.go:12`,
`needs-refinement` status, the SPA composer that was already
replaced by FEAT-064's range-comment UI) has zero usage in the
ledger. Across 178 done items, no item file contains a
`## Refinement history` block. The comment-driven auto-refine flow
shipped in FEAT-062/FEAT-064 covers every observed use case.

FEAT-065 closed with the keep-document decision. The data accumulated
since FEAT-065 (still 0 usage) makes the case for delete-not-document.

## Context

Surface to remove:

- `internal/server/items_refine.go` — handler.
- `/api/items/:id/refine` route registration.
- `internal/server/refine_list.go` (`/api/refine` GET handler) and
  the SPA panel that consumes it.
- `needs-refinement` status transitions and the doctor sweep over
  refinement-queued items.
- The legacy SPA composer leftovers if any survived FEAT-064.

What to verify before delete:

- `squad refine <ID>` CLI command — is it called by anything
  besides the now-deleted SPA POST? `plugin/skills/squad-loop`
  references it; that reference becomes stale and needs an edit.
  No external scripts in this repo call it (grep before delete).
- `squad recapture <ID>` CLI command pairs with `squad refine` for
  the closing of the peer-queue loop. Goes with refine if both
  paths drop.

## Acceptance criteria

- [ ] `/api/items/:id/refine` returns 404 (route removed). HTTP
      test pins the removal.
- [ ] `/api/refine` (the list endpoint) returns 404. Same.
- [ ] `needs-refinement` status no longer appears in the items
      state-machine; transitions to/from it are removed.
- [ ] The `Refinement claims (special case)` section in
      `plugin/skills/squad-loop` is removed or rewritten.
- [ ] A grep of the codebase for `needs-refinement`,
      `handleItemsRefine`, `handleRefineList` returns zero hits
      outside of `.squad/done/` historical item bodies and this
      item's own resolution.
- [ ] If `squad refine` / `squad recapture` CLI commands drop,
      AGENTS.md / CLAUDE.md are updated. If they stay, the
      "Refinement claims" doc clarifies they're the only path now.
- [ ] Tests: existing tests asserting `needs-refinement` behavior
      either drop or are repurposed to assert "the path no longer
      exists" (404, error string).

## Notes

- This is decisively a "remove dead code" item, not a behavior
  change. A grep-and-delete pass verified by the existing test
  suite catching anything that still depends on the path.
- The post-FEAT-064 SPA already routes "Send for refinement"
  through the comment-driven auto-refine flow, so user-facing
  surface change is zero.

## Resolution
(Filled in when status → done.)
