---
id: BUG-011
title: docs/reference/commands.md uses wrong learning kind names (actions/nots) — examples fail
type: bug
priority: P1
area: docs
status: done
estimate: 15m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241464
accepted_by: web
accepted_at: 1777241676
references: []
relates-to: []
blocked-by: []
---

## Problem

`docs/reference/commands.md` lines 433-440 document learning kinds as `actions`, `patterns`, `nots`. The actual implementation (`internal/learning/kind.go`) accepts only `gotcha`, `pattern`, `dead-end` — anything else returns `unknown kind %q (want gotcha | pattern | dead-end)`. The example commands `squad learning propose actions retry-on-503 …` and `squad learning propose nots dont-mock-the-db …` will fail immediately when a user copies them.

## Context

A user's first encounter with `squad learning` is the reference doc. Bad first-touch examples shake confidence in the rest of the docs. The same vocabulary drift exists in a stale comment at `internal/learning/learning.go:2` that lists `{actions,patterns,nots}` — that comment also needs updating. The directory layout in `internal/learning/paths.go` uses the plural forms `gotchas/patterns/dead-ends`, so the doc should use those for the on-disk paths and the singular forms for the kind argument to `squad learning propose`.

## Acceptance criteria

- [ ] `docs/reference/commands.md` "## Learning" section uses `gotcha`, `pattern`, `dead-end` for the kind argument and `gotchas/patterns/dead-ends` for directory names.
- [ ] Both example shell snippets in that section run successfully against a real squad install (no "unknown kind" error).
- [ ] Stale comment at `internal/learning/learning.go:2` updated to match.
- [ ] grep for `\b(actions|nots)\b` under `docs/` returns zero hits in the learning context (still fine in unrelated prose).

## Notes

Found during a parallel exploration sweep on 2026-04-26. Verified against `cmd/squad/learning.go:9` (Long string), `internal/learning/kind.go`, `internal/learning/paths.go`, and `cmd/squad/mcp_schemas.go:203`.

## Resolution
(Filled in when status → done.)
