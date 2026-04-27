---
id: FEAT-035
title: squad status surfaces multi-agent collision risk
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
blocked-by: []
parent_spec: agent-team-management-surface
epic: coordination-defaults-opinionated-opt-out
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

When more than one agent holds an active claim, `squad status` prints
a flat `claimed: N` line (`cmd/squad/status.go:119`) with no agent
attribution. The audit observed `claimed: 1` even when peers had work
in flight on the same repo, because `Status` only counts rows from
`SELECT item_id FROM claims WHERE repo_id = ?`
(`cmd/squad/status.go:73`) without surfacing `agent_id`. An operator
checking on contention has no way to see who holds what.

## Context

`Status` at `cmd/squad/status.go:48` is pure read aggregation — walk
items, query the claims table, fold counts. The query at line 73
already runs against the `claims` table; widening it to
`SELECT item_id, agent_id FROM claims WHERE repo_id = ?` gives the
breakdown for free. `StatusResult` (line 38) needs a sibling field
(e.g. `Holders []ClaimHolder` or `ClaimedBy map[string][]string`) so
the `--json` shape carries the breakdown without breaking the existing
keys. The plain-text printer at line 119 grows a conditional: when
`len(claimed) > 1`, indent a per-agent block underneath the existing
`claimed:` line.

Note: `--json` does not exist on `squad status` today
(`newStatusCmd` at line 18 has no flags). Adding it as part of this
item is fine — the StatusResult JSON tags are already in place.

## Acceptance criteria

- [ ] When `claimed > 1`, `squad status` prints a per-agent breakdown
      under the `claimed:` line, naming each agent and the item id(s)
      they hold. Single-claim and zero-claim output stays unchanged
      to avoid breaking existing scrape patterns.
- [ ] `squad status --json` (new flag) emits the same breakdown via a
      new field on `StatusResult` — existing fields keep their JSON
      tags untouched.
- [ ] A test exercises the multi-agent shape: two agents, two claims,
      stdout contains both agent ids and item ids.

## Notes

Sort the per-agent block deterministically (agent id ascending) so
test assertions and human eyes both stay stable. The
`internal/claims/` package owns the table; if reaching directly into
`db.QueryContext` from `status.go:73` feels off, push the breakdown
query into `internal/claims/claims.go` and call it from here.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
