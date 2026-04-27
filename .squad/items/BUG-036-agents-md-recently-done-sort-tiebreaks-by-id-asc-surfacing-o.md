---
id: BUG-036
title: AGENTS.md Recently done sort tiebreaks by ID asc, surfacing oldest BUGs over genuinely recent items
type: bug
priority: P2
area: internal/scaffold
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777323777
accepted_by: web
accepted_at: 1777325380
references: []
relates-to: []
blocked-by: []
---

## Problem

`pickDone` (`cmd/squad/scaffold_agents_md.go:115-128`) sorts done items
by `Updated` (the YAML date string `YYYY-MM-DD`) DESC, with `ID` ASC
as tiebreak. Per-day granularity means most done items in a busy
period share the same `Updated` value, so the secondary key
dominates: lexicographically-low IDs (`BUG-017`, `BUG-018`, …) win
over lexicographically-higher but genuinely more-recent ones
(`CHORE-015`, `FEAT-049`, `FEAT-050`, …). The "Recently done" section
then surfaces the oldest backlog items rather than what was just
shipped.

## Context

Today on main, `.squad/done/` has 101 items with `updated:
"2026-04-27"` and 34 with `2026-04-26`. The 10 lines rendered under
"## Recently done" are `BUG-017` through `BUG-027` — the lowest 10
BUG IDs in the same-day pile. The recently-completed FEAT-029…FEAT-032
auto-refine items, FEAT-049/FEAT-050/CHORE-015 documentation-contract
items, and FEAT-042/FEAT-051 cadence/listener items do not appear
even though all closed in that same window.

Reproduce:

```
go run ./cmd/squad scaffold agents-md && grep -A12 'Recently done' AGENTS.md
git checkout -- AGENTS.md
```

The header reads "Recently done" but the listed IDs are anything but.

## Acceptance criteria

- [ ] `pickDone` ranks items by a recency signal with sub-day
      resolution — the unix `accepted_at` field, or the
      `messages.created_at` of the `kind='done'` row, or file mtime —
      so the 10 most-recently-closed items in a multi-item day are
      the ones rendered.
- [ ] A regression test in `cmd/squad/scaffold_agents_md_test.go`
      pins ordering on a fixture where multiple items share an
      `updated` date but differ on the recency signal — the test
      fails on today's `Updated → ID` tiebreak.
- [ ] On the live ledger (after BUG-035 lands or independently), the
      generated AGENTS.md surfaces the FEAT-049 / FEAT-050 / CHORE-015
      cluster in "Recently done" rather than ten old BUGs.

## Notes

Cheapest signal that's already in `Item`: `AcceptedAt` (unix int64).
But many items pre-date the field being populated; for those, fall
back to `Updated` then `ID`. Documented in the code so future readers
know the fallback is intentional.
