---
id: BUG-030
title: touch path normalization mismatch between hook and squad touch CLI
type: bug
priority: P2
area: internal/touch
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777314534
accepted_by: web
accepted_at: 1777318793
references: []
relates-to: []
blocked-by: []
---

## Problem

The pre-edit-touch-check hook records absolute file paths (Claude Code
emits `tool_input.file_path` as absolute), but `squad touch` and
`squad claim --touches=...` are typically called with repo-relative
paths. `Tracker.Conflicts` and `ListOthers` compare on exact string
match (`internal/touch/touch.go` `WHERE path = ?`), so the same logical
file gets two distinct rows depending on which writer landed it — and
the warn/deny path silently misses conflicts in mixed-source workflows.

## Context

This gap pre-existed but became acute once FEAT-033 made the Edit/Write
hook the dominant writer. The fix is to normalize at write time: take
the repo root via `repo.Discover` (or pass it through), then
`filepath.Rel(root, path)` + `filepath.Clean`. Fall back to the absolute
path when the file is outside the repo (vendored deps, etc.).

## Acceptance criteria

- [ ] `Tracker.Add` and `Tracker.EnsureActive` normalize incoming paths
      to repo-relative form before insert, with a documented fallback
      for paths outside the repo.
- [ ] `Tracker.Conflicts`, `Tracker.Release`, and `Tracker.ListOthers`
      apply the same normalization to the lookup key.
- [ ] Regression test: the same logical file written via the hook
      (absolute) and via `squad touch` (relative) collide on the
      conflict query.

## Notes

Discovered during code review of FEAT-033. The reviewer flagged this
as "important, file follow-up" rather than blocking that change.
Touching internal/touch keeps the blast radius contained — callers do
not need to know.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
