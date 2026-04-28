---
id: CHORE-018
title: get api items should accept repo_id filter for parity with post
type: chore
priority: P3
area: server
status: open
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777340773
accepted_by: web
accepted_at: 1777340931
references: []
relates-to: []
blocked-by: []
---

## Problem

`POST /api/items` honors `?repo_id=` (workspace-mode create routes the file
to the named repo). The companion read endpoint `GET /api/items` does not —
it accepts `status` and `epic` filters but ignores `repo_id`, so callers
that want a repo-scoped slice of the workspace have to pull every item and
client-filter on the `repo_id` field. The new-item modal's integration test
(`TestHandleItemsCreate_WorkspaceMode_TwoReposNoLeakage`) settled for
verifying per-row tags on the un-filtered list as a proxy for the
repo-scoped query that the wire contract should support.

## Context

`internal/server/items.go` `handleItemsList` reads
`r.URL.Query().Get("status")` and `Get("epic")` around line 134; mirroring
that pattern for `repo_id` is a 3-line change. Affects:

- The SPA, when it eventually grows a per-repo scope (today the dashboard
  aggregates everything; the new-item modal's pre-selection guess uses
  first-by-id from `/api/repos`).
- The TUI client (`internal/tui/client`), which speaks the same wire.
- MCP / external callers that scope ad-hoc queries.

## Acceptance criteria

- [ ] `GET /api/items?repo_id=<X>` returns only items tagged with `<X>`,
      preserving status/epic filter composition.
- [ ] Single-repo mode (where `cfg.RepoID != ""`) ignores or short-circuits
      the param without breaking existing callers.
- [ ] Workspace-mode test pins both branches: filter narrows the list when
      `repo_id=repo-A`, and an unknown repo_id yields zero rows (not 404).

## Notes

Surfaced during code review of the workspace-mode new-item modal change.
The modal's test verifies "no cross-repo leakage" via per-row tag
inspection on the un-filtered list, which covers intent but not the
literal AC wording in the parent item; a `repo_id` filter on the GET
closes that gap and unblocks repo-scoped consumers.

## Resolution
(Filled in when status → done.)
