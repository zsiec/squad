---
id: BUG-002
title: web UI dashboard shows 0 agents in AGENTS panel even when agents are registered and have active claims
type: bug
priority: P1
area: server
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777237210
accepted_by: agent-bbf6
accepted_at: 1777237210
references: []
relates-to: []
blocked-by: []
---

## Problem
The web UI dashboard's AGENTS panel (left sidebar) shows `0` agents even when
agents are registered in the squad and actively holding claims. Reproduced on
2026-04-26 ~16:58 local: `agent-bbf6` (this session) and `agent-401f` (Malik
Moore, holding BUG-001) were both registered, yet the AGENTS panel header
read "0" and the list was empty. The header counters above (0 ACTIVE / 1 IN
FLIGHT) and the in-progress table (showing BUG-001 claimed by Mal...)
correctly reflected agent activity, so the data is reaching the server —
only the AGENTS panel rendering / data feed is broken.

## Context
The dashboard SPA lives at `internal/server/web/`. The agent roster is
served by the dashboard HTTP API in `internal/server/`. Need to determine
whether (a) the API endpoint that feeds the AGENTS panel is returning an
empty list, (b) the SSE/refresh path isn't pushing roster updates, or (c)
the SPA is filtering agents out client-side (e.g. requiring a non-empty
display name or filtering inactive agents too aggressively). The other
panels — header counters and in-progress claim — are populated correctly,
suggesting the underlying squad state is fine and this is a panel-specific
bug.

## Acceptance criteria
- [ ] Reproduce: run `squad serve` (or equivalent), register two agents,
      have one claim an item, confirm AGENTS panel currently shows `0`.
- [ ] Identify the root cause (API response, SSE delta, client filter).
- [ ] AGENTS panel lists every registered agent with a non-zero count in
      its header, matching the agents the server knows about.
- [ ] Newly-registered agents appear in the panel within one refresh cycle
      without requiring a page reload.
- [ ] Unit or integration test covers the agent-roster path for the panel.

## Notes
- Captured during a `/squad:squad-work` session where workspace was clear
  and `agent-401f` had just claimed BUG-001 (visible via global chat
  mailbox flush).
- Screenshot evidence in user-supplied attachment showing AGENTS=0 while
  BUG-001 is in-flight under `Mal...`.
- Watch for interaction with BUG-001 (claims PK collision) — that fix
  may touch agent identity / per-repo scoping; coordinate ordering if so.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
