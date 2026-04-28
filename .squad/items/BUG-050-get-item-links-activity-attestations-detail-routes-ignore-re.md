---
id: BUG-050
title: GET item/links/activity/attestations detail routes ignore ?repo_id= for cross-repo collisions
type: bug
priority: P3
area: internal/server
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-afcd
captured_at: 1777336251
accepted_by: web
accepted_at: 1777336308
references: []
relates-to: []
blocked-by: []
---

## Problem

In workspace mode, four GET-by-item-id routes do not accept a
`?repo_id=` query param to disambiguate when the same item id exists
in more than one repo:

- `GET /api/items/{id}` → silently picks the first match in
  `walkAll` iteration order (sort key is repo_id). User has no way
  to say "I want BUG-001 from repo A, not repo B."
- `GET /api/items/{id}/links` → same `walkAll` shape, same
  silent-pick behavior.
- `GET /api/items/{id}/activity` → queries the chat/messages table
  by item id; with `cfg.RepoID == ""` there is no repo filter, so
  events from BUG-001 across every repo come back interleaved.
- `GET /api/items/{id}/attestations` → calls `attest.ListForItem`
  with `cfg.RepoID == ""`, which (correctly per BUG-042) returns
  records from all repos but offers no client-side filter.

The mutation routes are explicit about this — BUG-044 wired
`resolveItemRepo` to return 409 `AmbiguousRepoError` and accept
`?repo_id=` as the disambiguator. The read-detail routes diverge:
they silently aggregate or pick. Result: a user clicking into a
detail panel for an item that happens to collide across repos sees
data from the wrong repo (or mixed) without any signal that
disambiguation was needed.

## Context

Reproduce against a workspace-mode daemon with at least two repos
that share an item id (synthetic case — squad ids rarely collide
in practice, but two checkouts of the same repo definitely do):

```
$ curl -s 'http://<workspace-daemon>/api/items/BUG-001'
# returns the first repo's BUG-001, no indication a second exists

$ curl -s 'http://<workspace-daemon>/api/items/BUG-001?repo_id=<other>'
# query param is ignored — returns the same first match
```

Compare to the mutation route shape:

```
$ curl -X POST '...api/items/BUG-001/claim'
# 409 Conflict {"error":"item BUG-001 exists in multiple repos [A B]; pass ?repo_id= to disambiguate"}

$ curl -X POST '...api/items/BUG-001/claim?repo_id=A'
# 204 No Content
```

`handleSpecDetail` (`internal/server/specs.go:75-114`) and
`handleEpicDetail` already accept `?repo_id=`. The four item-detail
routes did not get the same treatment.

## Acceptance criteria

- [ ] `handleItemDetail`, `handleItemLinks`, `handleItemActivity`,
      `handleAttestationsForItem` each read `?repo_id=` from the
      query string. When set in workspace mode, scope results to
      that repo; when unset and the id is ambiguous, return 409
      with the same `AmbiguousRepoError` shape `resolveItemRepo`
      uses for mutation routes.
- [ ] `handleItemDetail`'s response gains a `repo_id` field at
      the top level (it already comes back nested in some paths;
      pin it on the wire shape).
- [ ] A test in `internal/server/` seeds two repos with the same
      item id and exercises each of the four routes with and
      without `?repo_id=`, asserting (a) ambiguous → 409, (b)
      with the param → scoped result, (c) single-repo mode
      unchanged (no 409, no extra param required).

## Notes

Single-repo mode (`cfg.RepoID != ""`) must be byte-identical to
today — the disambiguation logic only kicks in when `cfg.RepoID == ""`.

Lower priority than BUG-048/049 because the failure mode here is
contingent on item-id collision across repos, which is rare in
practice. The fix is consistency with the mutation routes more than
a critical correctness bug.
