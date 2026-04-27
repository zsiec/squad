---
id: FEAT-034
title: squad claim warns on overlapping touches from peers
type: feature
priority: P2
area: cmd/squad
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309557
references: []
relates-to: []
blocked-by: [FEAT-033]
parent_spec: agent-team-management-surface
epic: coordination-defaults-opinionated-opt-out
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

When another agent has a fresh touch on a file the new claim's item
references, `squad claim` should surface the conflict at claim time so
the agent can `squad knock` the peer before editing. Today claim is
silent on peer activity unless the agent passes `--touches`, and
even then the conflict surfaces only on the touched files, not on
files the item already declared.

## Context

`cmd/squad/claim.go:192` is the `Claim` entrypoint; the success path
runs from line 206 with several existing nudges
(`printCadenceNudge`, `printSecondOpinionNudge`,
`printMilestoneTargetNudge`, `printWorktreeNudge`). The peer-touch
warning is a fifth nudge of the same shape, run before returning.
The query API is `touch.Tracker.ListOthers`
(`internal/touch/touch.go:122`), which already filters out the
caller's own touches and returns peer (agent, item, path, started_at).
A 24h freshness filter belongs in the query — extend ListOthers or
add a sibling. The "files the item references" set is already parsed
on the success path: `items.Parse` at `cmd/squad/claim.go:210` returns
the item body, and `references` frontmatter (plus optional path
extraction from acceptance criteria) gives us the comparison set.

This depends on FEAT-033 because `ListOthers` returns nothing useful
until the Edit/Write hook actually writes touch rows from peer
sessions.

## Acceptance criteria

- [ ] `squad claim` queries peer touches written in the last 24h via
      `touch.Tracker.ListOthers` (or a freshness-bounded sibling).
- [ ] If any peer touch overlaps a file the claimed item references,
      print a one-line warning to stderr naming the peer agent and
      file: `squad: heads up — agent-XXXX is touching <path> (last 4h)`.
- [ ] Warning is informational only — claim still succeeds, exit code
      stays 0. Coordination is the agent's call (matching the existing
      nudge pattern at `cmd/squad/claim.go:208`).

## Notes

Keep the warning to a single line per overlap; if there are >3
overlaps, collapse to "and N more — squad touches list-others to see
all". Don't gate on warning silence in tests — assert on stderr
substring, not on exit code (it's already 0).

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
