---
id: FEAT-022
title: 'intake: commit spec/epic/items bundles atomically'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 2h
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290819
accepted_by: web
accepted_at: 1777291152
references: []
relates-to: []
blocked-by: [FEAT-021]
---

## Problem
Full-hierarchy bundles need to commit a spec, ≥1 epic, and ≥1 item per epic in one shot, with all three layers linked.

## Context
Builds on FEAT-021. Write order: spec → epics → items. Each artifact:
- Spec/epic markdown gains `intake_session: <id>` frontmatter line.
- Epic frontmatter `spec: <spec-slug>`.
- Items get `parent_spec`, `epic_id` (or `parent_epic`), `intake_session_id` populated.

Plan ref: Task 9.

## Acceptance criteria
- [ ] Happy path: spec file + epic files + item files on disk; spec/epic/item rows in DB.
- [ ] Every item row has `parent_spec` set to the spec slug.
- [ ] Every item row has `epic_id` matching the correct epic slug.
- [ ] Slug conflict on the spec rolls back: nothing written, no rows.
- [ ] Tests cover happy path (verify all three FK linkages), and slug-conflict rollback.

## Notes
Reuse atomicity machinery from FEAT-021 — same tx, same deferred cleanup approach, just more files.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
