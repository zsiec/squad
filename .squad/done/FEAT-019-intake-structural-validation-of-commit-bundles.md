---
id: FEAT-019
title: 'intake: structural validation of commit bundles'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 1h30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290812
accepted_by: web
accepted_at: 1777291152
references: []
relates-to: []
blocked-by: [FEAT-014]
---

## Problem
The commit bundle must be validated structurally before any file or row is written. This is the real gate that catches half-baked bundles.

## Context
First half of `internal/intake/commit.go`. Pure function: `Validate(bundle, mode, refineItemID, checklist) (shape string, err error)`. Detects `item_only` vs `spec_epic_items` from bundle structure, walks required fields, asserts non-empty title, ≥1 acceptance bullet, slug-safe-derived names, refine-mode constraints (single item, no spec, no epics).

Plan ref: Task 6.

## Acceptance criteria
- [ ] Detects shape correctly for `item_only` and `spec_epic_items`.
- [ ] Returns `IntakeShapeInvalid` for malformed mixes (spec without epics, etc.).
- [ ] Returns `IntakeIncomplete{Field}` naming the offending field.
- [ ] In refine mode, asserts exactly one item, no spec, no epics.
- [ ] Table-driven tests cover ≥10 cases including all required-field omissions and malformed shapes.

## Notes
No DB access in this task — pure validator. Slug conflict check is FEAT-020.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
