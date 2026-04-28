---
id: BUG-044
title: extend workspace mode to mutation routes (auto-refine, recapture, claim, done, accept, refine, reject, blocked, force-release, items_create, handoff)
type: bug
priority: P1
area: internal/server
status: done
estimate: 2h
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777332416
accepted_by: ""
accepted_at: 0
references: []
relates-to: []
blocked-by: []
---

## Problem

When the dashboard daemon runs in workspace mode (cwd has no squad repo, so `repo.Discover` fails and `cfg.RepoID == ""`), every mutation route still calls `items.FindByID(s.cfg.SquadDir, id)` or `items.<Mutation>(..., s.cfg.RepoID, ...)` against an empty/garbage scope. The lookup returns `ErrItemNotFound` and the SPA shows "item not found" on auto-refine, recapture, claim, done, etc., even though the read endpoints (post BUG-040) correctly aggregate the item across all repos.

Reproduce: `curl -X POST http://127.0.0.1:7777/api/items/BUG-041/auto-refine -d '{}' -H 'Content-Type: application/json'` against a launchd-managed daemon → 404 `{"error":"item not found"}`.

## Context

BUG-040 (`feat(serve): workspace mode for items / agents / inbox / claims`) wired workspace mode for the read paths via `walkAll` enumerating `repos` table → walking each `<root>/.squad/`. That commit deliberately stopped short of mutation routes; BUG-040's done summary explicitly says "Mutation routes and specs/epics walks remain single-repo." BUG-042 covers the remaining read routes; this item covers mutations. Together they finish the workspace-mode picture.

Affected handlers (each does either `FindByID(squadDir, id)` or passes `cfg.RepoID` into a mutation helper):

- `internal/server/items_auto_refine.go` (FindByID)
- `internal/server/items_recapture.go` (FindByID)
- `internal/server/items_accept.go` (helper takes squadDir)
- `internal/server/items_reject.go` (squadDir)
- `internal/server/items_create.go` (squadDir)
- `internal/server/claim.go` (squadDir + repoID)
- `internal/server/done.go` (squadDir + repoID)
- `internal/server/blocked.go` (findItemPathFor + squadDir)
- `internal/server/handoff.go` (likely; verify)
- `internal/server/force_release.go` (likely; verify)

## Design question to resolve before coding

A mutation needs to know which repo the item lives in (so the per-repo `.squad/items/<ID>.md` rewrite and the per-repo `claims` row hit the right scope). Two options:

1. **Resolve repo from `items` table.** `SELECT repo_id, path FROM items WHERE item_id = ?` on the global DB. Authoritative, no client cooperation needed. Downside: if two repos have the same item ID (BUG-001 in repo A and repo B), the query returns 2 rows — need a tiebreak (latest update? error and require disambiguation?).
2. **SPA passes `?repo_id=` on the request.** The SPA has `repo_id` on every row from the read aggregation. Cleaner separation; no server-side ambiguity. Downside: legacy clients that never send `repo_id` break in workspace mode (acceptable — they're already broken today).

Recommendation: do **both** — server resolves from `items` table when `repo_id` is absent, errors with 409 + "ambiguous: pass ?repo_id=" when multiple match. Single-repo mode (`cfg.RepoID != ""`) still preserves today's behavior.

## Acceptance criteria

- [ ] In workspace mode, POST `/api/items/{id}/auto-refine` against a known item returns 200 (or 503 if claude CLI absent) instead of 404
- [ ] Same for recapture, accept, refine, reject, blocked, force-release, claim, done, handoff
- [ ] Single-repo mode (`cfg.RepoID != ""`) behavior is unchanged for every mutation route — pinned by the existing single-repo tests
- [ ] When two repos contain the same item ID and the request omits `?repo_id=`, the server returns 409 with a body naming both candidate repos
- [ ] Each mutation route has a workspace-mode test in `internal/server/*_test.go` that seeds a multi-repo fixture global DB (mirroring BUG-040's `TestHandleItemsList_WorkspaceModeAggregatesRepos` shape) and asserts the mutation reaches the right per-repo file
- [ ] `items_create` in workspace mode requires `?repo_id=` (no implicit "first repo" behavior)

## Notes

- BUG-042 (concurrent) covers read routes — it should not touch mutation handlers, and this item should not touch read handlers
- The `commitlog.ListForItem(db, repoID, id)` call in `links.go` is read-side and belongs to BUG-042
- Per agent-bbf6's design note on BUG-042: "epic and spec slugs are per-repo unique, not workspace-global." The same applies here — item IDs *appear* unique by accident of squad's monotonic numbering, not by guarantee. The 409-on-collision behavior protects against that latent bug surfacing later.
