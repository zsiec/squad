---
id: BUG-049
title: GET /api/learnings ignores workspace mode — walks one repo's learnings dir, never aggregates across repos
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
captured_at: 1777336216
accepted_by: web
accepted_at: 1777336305
references: []
relates-to: []
blocked-by: []
---

## Problem

`handleLearningsList` and `handleLearningDetail`
(`internal/server/learnings.go:22-96`) call `learning.Walk(s.cfg.LearningsRoot)`
and `learning.ResolveSingle(s.cfg.LearningsRoot, slug)`. `LearningsRoot`
is a single filesystem path; in workspace mode it's set from
`repo.Discover(wd)` in `cmd/squad/serve.go:165`, which falls back to
empty when discover fails, then `server.New` defaults it to `"."`
(`internal/server/server.go:54-55`). The result: the daemon walks its
own CWD instead of aggregating learnings across all registered repos,
so the SPA's learnings tab is empty (or shows only the daemon's CWD's
learnings) on a workspace-mode dashboard.

The other two read-side aggregations BUG-042 wired (specs, epics)
already enumerate the `repos` table and walk each `<root>/.squad/`.
Learnings missed that sweep.

## Context

Reproduce against a workspace-mode daemon (one started where
`repo.Discover` failed, so `cfg.RepoID == ""` and
`cfg.LearningsRoot == "."`):

```
$ curl -s http://<workspace-daemon>/api/learnings
[]
$ # despite multiple registered repos having learnings:
$ ls /Users/zsiec/dev/squad/.squad/learnings/
proposed/  approved/  rejected/  ...
```

Source of the divergence:

```go
// internal/server/learnings.go:22-23
func (s *Server) handleLearningsList(w http.ResponseWriter, r *http.Request) {
    all, err := learning.Walk(s.cfg.LearningsRoot)
```

Compare to the workspace-aware spec walker:

```go
// internal/server/specs.go:22-56 — walkSpecsAll
func (s *Server) walkSpecsAll() ([]specWithRepo, error) {
    if s.cfg.RepoID != "" {
        return specs.Walk(s.cfg.SquadDir) // single-repo
    }
    rows, _ := s.db.Query(`SELECT id, COALESCE(root_path, '') FROM repos ORDER BY id`)
    // ... walk each repo's .squad/specs ...
}
```

## Acceptance criteria

- [ ] `handleLearningsList` aggregates learnings across every
      registered repo in workspace mode (`cfg.RepoID == ""`),
      mirroring the `walkSpecsAll`/`walkEpicsAll` shape: enumerate
      `repos` table, walk each `<root_path>/.squad/learnings/`,
      tag each result with the repo id.
- [ ] `handleLearningDetail` accepts an optional `?repo_id=` query
      param to disambiguate when the same slug appears in multiple
      repos (matches `handleSpecDetail`/`handleEpicDetail`).
- [ ] Each `learningRow` JSON object grows a `"repo_id"` field
      (omitempty for single-repo backward compatibility).
- [ ] A test in `internal/server/` seeds two repos with at least
      one learning each, runs `handleLearningsList` with
      `cfg.RepoID == ""`, and asserts both come back tagged with
      their repo ids.

## Notes

Cheapest fix: extract a `walkLearningsAll` helper modeled on
`walkSpecsAll`/`walkEpicsAll`. The single-repo fast path
(`learning.Walk(cfg.LearningsRoot)`) preserves byte-identical
behavior for non-workspace daemons.

Worth checking whether the legacy fallback `cfg.LearningsRoot = "."`
in `internal/server/server.go:54-55` still has any callers after
this fix — it was originally there to keep tests working with a
relative CWD. After the workspace branch lands, that default may be
dead code worth deleting, or worth keeping as a safety net for the
`SquadDir` variant — judgement call for the implementer.
