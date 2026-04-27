---
id: FEAT-048
title: squad stats by capability
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
accepted_at: 1777309559
references: []
relates-to: []
blocked-by: [FEAT-045]
parent_spec: agent-team-management-surface
epic: capability-routing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Once capability routing is in place, operators need to know which
capabilities are saturated and which are starved. Without a
breakdown, you cannot tell whether `frontend` work is piling up
because no agent has the tag, or because the work is genuinely
hard. The default `squad stats` view groups by item type, not by
capability.

## Context

`internal/stats/` owns the aggregation passes; `cmd/squad/stats.go`
owns the CLI surface. Add a `--by capability` mode that:

- Reads completed items and their `requires_capability` tag set.
- Counts each completion once per tag (an item tagged
  `["go", "sql"]` increments both buckets).
- Respects the existing `--since` window flag for time-bounded
  views.

This depends on FEAT-045 only — the column has to exist and be
parseable. FEAT-046 (agent-side tags) and FEAT-047 (filter) are not
strict prerequisites; the stat reads item-side data and is useful
even before any agent registers a capability set.

## Acceptance criteria

- [ ] `squad stats --by capability` prints a per-tag count of done
  items.
- [ ] Items tagged with multiple capabilities increment each tag's
  count once.
- [ ] Items with empty `requires_capability` are reported under a
  `(untagged)` bucket or omitted — pick the option that reads
  cleanly and document the choice.
- [ ] `--since <duration>` narrows the window using the same
  semantics as the existing stats commands.
- [ ] Test covers single-tag, multi-tag, untagged, and time-window
  cases.

## Notes

Counting once per tag (rather than splitting fractionally) keeps the
output readable: each row answers "how many done items needed this
capability?" The trade-off is that totals across rows exceed the
total done count whenever any item has multiple tags — call that
out in the column header or help text.

## Resolution

(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
