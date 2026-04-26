---
id: TASK-013
title: squad new templates evidence_required:[test] for bug/feat/task
type: task
priority: P2
area: items
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-bbf6
captured_at: 1777245991
accepted_by: agent-bbf6
accepted_at: 1777245991
epic: feature-uptake-nudges
evidence_required: [test]
references:
  - internal/items/new.go
relates-to: []
blocked-by: []
---

## Problem

`squad new BUG ...` writes a stub item with no `evidence_required` field. Even after TASK-012 lights up the config-level fallback, the templated frontmatter is the surface humans see and edit; if it doesn't show the field, it doesn't exist as a concept for the author. Default the field on for the three types where test attestation is genuinely the right bar (bug, feat, task); leave it `[]` for chore/tech-debt/bet.

## Context

The stub template lives at `internal/items/new.go:24-50` (`stubTemplate` constant). The `NewWithOptions` function at the bottom of the same file does the `Sprintf` — adding one more `%s` and a small helper that maps prefix → default kinds is the whole change.

## Acceptance criteria

- [ ] `internal/items/new.go` `stubTemplate` includes a new line `evidence_required: %s` (positioned after `risk:` and before `created:`).
- [ ] A new helper `defaultEvidenceForType(prefix string) string` returns `"[test]"` for `BUG`/`FEAT`/`TASK`, `"[]"` for `CHORE`/`DEBT`/`BET` and any other prefix.
- [ ] `NewWithOptions` passes the helper's result to the `Sprintf` call.
- [ ] Unit test: `New(tmp, "BUG", "x")` produces a body containing `evidence_required: [test]`.
- [ ] Unit test: `New(tmp, "CHORE", "x")` produces a body containing `evidence_required: []`.
- [ ] `go test ./internal/items/...` passes; trailing `ok` line pasted into close-out chat.

## Notes

- Independent of TASK-012 — this can land before or after, in any order.
- Existing items in `.squad/items/` are not retroactively rewritten; that's deliberate (no item file mutation), and the config fallback from TASK-012 covers them.
- We don't add a `--evidence-required` CLI flag yet; the template default plus hand-edit is enough.

## Resolution

(Filled in when status → done.)
