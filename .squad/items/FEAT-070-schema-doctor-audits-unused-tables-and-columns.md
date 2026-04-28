---
id: FEAT-070
title: schema doctor audits unused tables and columns
type: feature
priority: P3
area: store
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352041
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
---

## Problem

The DB schema has bit-rot. During the polish-and-prune audit
several anomalies surfaced:

- A `learnings` table is referenced in skill prose but does not
  exist in the schema; learnings live as filesystem artifacts.
- A `reads` table exists with columns that don't match expected
  shape (no `ts`); unclear whether it's still written.
- A `progress` table exists with columns the dashboard surfaces
  but querying it produced "no such column: phase" — schema and
  call sites are out of sync.
- `wip_violations` has 4 rows total, ever; likely a leftover.

This isn't a critical bug — the live system still works — but it's
exactly the kind of drift that makes a future "extend the data
model" change hazardous.

## Context

A standalone `squad schema-doctor` (or
`squad doctor --schema`) verb walks every table in
`internal/store/schema.sql` and reports:

- Tables with zero rows, zero inserts in 30d, and no read call
  sites in `internal/`. Recommend removal.
- Tables present in code references but missing from the schema.
  Recommend adding or removing the reference.
- Columns referenced in code but absent from the schema (or
  vice versa). Recommend the missing migration.

Output is informational, not destructive. No automatic schema
changes.

## Acceptance criteria

- [ ] `squad schema-doctor` runs and emits a markdown report
      naming each anomaly with file:line references for code
      callers.
- [ ] The report distinguishes (a) tables with no recent
      activity, (b) tables/columns referenced in code but not in
      schema, (c) tables/columns in schema but not in code.
- [ ] Exit code is 0 on no findings; non-zero with a summary
      otherwise (so it can gate a CI check if desired).
- [ ] Tests: fixture schema + fixture code references that
      reproduce each of the three anomaly classes; assert the
      report names them correctly.

## Notes

- Companion to FEAT-069 (`squad doctor --feature-usage`). They
  serve different audiences (schema sweep is for whoever's
  maintaining the data model; feature usage is for the operator).
  Keep separate verbs to avoid one-flag-tells-everything.
- Read-only against the production DB; safe to run anytime.

## Resolution
(Filled in when status → done.)
