---
id: BUG-048
title: GET /api/refine ignores workspace mode — returns nothing when cfg.RepoID is empty
type: bug
priority: P2
area: internal/server
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-afcd
captured_at: 1777336184
accepted_by: web
accepted_at: 1777336303
references: []
relates-to: []
blocked-by: []
---

## Problem

`handleRefineList` (`internal/server/refine_list.go:13-34`) passes
`s.cfg.RepoID` directly into the SQL filter without the workspace-
mode `""` sentinel branch every other read route now uses. In
workspace mode (`cfg.RepoID == ""`) the query becomes
`WHERE repo_id='' AND status='needs-refinement'`, which matches no
rows. The needs-refinement queue is invisible in the SPA and
`squad refine` (CLI) when called against a workspace daemon.

Same shape as the original BUG-040 inbox bug — missed by both
BUG-040 (items / agents / inbox / claims) and BUG-042 (the second
read-route sweep).

## Context

Reproduce against the running workspace-mode daemon (or any daemon
where `cfg.RepoID == ""`):

```
$ curl -s http://127.0.0.1:18555/api/refine
[]
$ # but the items table has needs-refinement rows somewhere:
$ sqlite3 ~/.squad/global.db \
    "SELECT item_id, repo_id FROM items WHERE status='needs-refinement'"
```

Source of the divergence:

```go
// internal/server/refine_list.go:14-18
rows, err := s.db.QueryContext(r.Context(),
    `SELECT item_id, title, COALESCE(captured_by,''), COALESCE(captured_at,0), COALESCE(updated_at,0)
     FROM items WHERE repo_id=? AND status='needs-refinement'
     ORDER BY updated_at ASC`,
    s.cfg.RepoID)
```

Compare to the inbox handler post-BUG-040:

```go
// internal/server/inbox.go:16-29
if s.cfg.RepoID == "" {
    rows, err = s.db.QueryContext(...
        `... FROM items WHERE status='captured' ORDER BY captured_at ASC`)
} else {
    rows, err = s.db.QueryContext(...
        `... FROM items WHERE repo_id=? AND status='captured' ORDER BY captured_at ASC`,
        s.cfg.RepoID)
}
```

## Acceptance criteria

- [ ] `handleRefineList` widens the SQL when `s.cfg.RepoID == ""`
      to omit the `repo_id` filter, mirroring the inbox handler shape.
- [ ] Each returned `refineEntry` carries the item's `repo_id` so
      the SPA can disambiguate when the same item id appears in
      multiple repos (mirrors `api.InboxEntry.RepoID`).
- [ ] A test in `internal/server/` seeds two repos with
      `needs-refinement` rows, runs the handler with `cfg.RepoID == ""`,
      and asserts both rows come back tagged with their repo ids.
- [ ] `curl -s http://<workspace-daemon>/api/refine` against the
      live daemon returns the expected rows after the fix.

## Notes

Cheapest fix: copy the inbox handler's `if cfg.RepoID == ""`
shape verbatim and add `repo_id` to the SELECT + struct.
