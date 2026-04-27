---
id: FEAT-044
title: squad stats surfaces done-to-release ratio per agent
type: feature
priority: P2
area: internal/stats
status: done
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
- `internal/stats/schema.go`: `AgentRow` gains `ReleaseCount int64` and `Ratio
  *float64`. Ratio is nil when ReleaseCount is zero — operators' undefined
  case is intentionally distinct from a low ratio.
- `internal/stats/breakdowns.go`: widened the claim_history query to count
  both `done` and `released` outcomes via SUM(CASE WHEN); kept the duration
  GROUP_CONCAT scoped to done-only so percentile math is unaffected. The
  cap-and-spill into `_other` now rolls up ReleaseCount and recomputes Ratio
  on the spill row, so >50-agent repos don't silently drop releases.
- `cmd/squad/stats.go`: new `--by` flag (only valid value: `agent`); when
  set, renders a focused table with columns `agent / done_count /
  release_count / ratio`, sorted by ratio DESC with nil-ratio agents last.
  Zero-release agents render ratio as `-`.
- Counted only voluntary `released` outcomes — `force_released` and
  `reclaimed` are operator/reaper-driven and would corrupt the per-agent
  quality signal. Documented inline on the field.
- Tests: `internal/stats/breakdowns_test.go` adds
  `TestByAgentDoneReleaseRatio` (the 6/2/3.0, 4/0/nil, 1/4/0.25 cases) and
  `TestByAgentSpillRollsUpReleaseCount` (60 agents → 50 visible + `_other`
  with 10/10/1.0). `cmd/squad/stats_test.go` adds
  `TestRenderAgentRatioTable_OrderingAndZeroRelease` (header columns, sort
  order, `-` for zero-release) and `TestStatsByAgentFlagRejectsUnknownGroup`
  (clear error on bogus `--by`).
- Schema version not bumped: adding fields with omittable zero-values is
  additive, not breaking, and the existing comment only flags
  rename/remove/semantics-change as breaking.
