---
id: TASK-011
title: docs + skill addendum for refinement loop
type: task
priority: P3
area: docs
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-1f3f
captured_at: 1777242010
accepted_by: agent-1f3f
accepted_at: 1777242010
epic: inbox-refinement
references:
  - plugin/skills/squad-loop/SKILL.md
  - internal/scaffold/templates/AGENTS.md.tmpl
  - docs/recipes/
relates-to: []
blocked-by:
  - TASK-006
  - TASK-007
  - TASK-010
---

## Problem

Without docs, the refinement loop is invisible to agents who weren't around when it shipped. The squad-loop skill, the AGENTS.md template, and a new recipe all need a paragraph each.

## Context

Three files to touch:

- `plugin/skills/squad-loop/SKILL.md` — short addendum: refinement claims skip TDD, evidence gates, and `squad done`. The verbs are `squad refine` → `squad claim` → edit → `squad recapture`.
- `internal/scaffold/templates/AGENTS.md.tmpl` — one-line pointer in the loop summary.
- `docs/recipes/refining-captured-items.md` (new) — short recipe explaining the reviewer flow and the agent flow.

Watch the token budget regression test in `internal/scaffold/` — small changes only.

## Acceptance criteria

- [ ] `plugin/skills/squad-loop/SKILL.md` has a "Refinement claims (special case)" section.
- [ ] `internal/scaffold/templates/AGENTS.md.tmpl` references the refinement verbs in the loop summary.
- [ ] `docs/recipes/refining-captured-items.md` exists.
- [ ] `go test ./internal/scaffold/ -count=1` passes (token budget regression intact); output pasted.

## Notes

Last item in the epic. Land after the verbs (TASK-006, TASK-007) and the integration test (TASK-010) so the docs describe what actually shipped.
