---
id: FEAT-014
title: 'intake: schema and required-fields checklist'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290806
accepted_by: web
accepted_at: 1777290850
references: []
relates-to: []
blocked-by: []
---

## Problem
The interview tool needs persistence (sessions + turns) and a configurable required-fields checklist. Neither exists today.

## Context
Migration 008 is the highest. New migration 009 introduces `intake_sessions`, `intake_turns`, plus `items.intake_session_id`. The checklist lives in a new `internal/intake` package — embedded YAML with optional per-repo override at `.squad/intake-checklist.yaml`.

Plan ref: Task 1 in `~/dev/switchframe/docs/plans/2026-04-27-intake-interview.md`.

## Acceptance criteria
- [ ] Migration 009 applies cleanly on a fresh DB and on an existing DB at migration 008.
- [ ] `intake_sessions` includes a partial unique index `(repo_id, agent_id) WHERE status='open'`.
- [ ] `intake_turns` enforces unique `(session_id, seq)`.
- [ ] `items.intake_session_id` column is nullable; existing rows unaffected.
- [ ] `internal/intake/checklist.go` exposes `LoadChecklist(squadDir)` that prefers `.squad/intake-checklist.yaml` and falls back to embedded.
- [ ] Tests cover: embedded default parses; per-repo override precedence; missing-required detection per shape.

## Notes
Companion design + plan are gitignored at `~/dev/switchframe/docs/plans/2026-04-27-intake-interview-{design,}.md`.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
