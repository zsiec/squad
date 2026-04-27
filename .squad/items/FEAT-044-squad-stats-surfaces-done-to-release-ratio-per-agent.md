---
id: FEAT-044
title: squad stats surfaces done-to-release ratio per agent
type: feature
priority: P2
area: internal/stats
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309558
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: cadence-and-time-boxing-as-pacing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

There is no per-agent quality signal in `squad stats`. The ratio of
`done` (closed with success) to `release` (dropped without close) tells
operators who is finishing vs spinning, and the data is already in the
claim history table — it just is not surfaced.

## Context

- `internal/stats/` holds the existing aggregation; the new group keys
  off the same query layer.
- `cmd/squad/stats.go` is the CLI surface that needs the new `--by
  agent` flag.
- The `claim_history` table records both `done` and `release`
  outcomes, so no schema change is needed.

## Acceptance criteria

- [ ] `squad stats` grows a `--by agent` group with `done_count`,
      `release_count`, and `ratio` columns.
- [ ] Default ordering is by `ratio` descending.
- [ ] Ratio is undefined for agents with zero releases — render as `-`.
- [ ] Aggregation respects the existing time-window flags on
      `squad stats` (no special-casing the new group).
- [ ] Test coverage includes the zero-release case and the standard
      ordering case.

## Notes

Keep the column header literally `ratio` — operators eyeballing the
output should not need to decode an abbreviation. The undefined case is
intentionally `-` rather than `0` or `NaN`; zero releases is a different
signal from a poor ratio.

## Resolution

(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
