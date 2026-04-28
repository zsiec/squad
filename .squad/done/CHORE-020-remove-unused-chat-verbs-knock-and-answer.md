---
id: CHORE-020
title: remove unused chat verbs knock and answer
type: chore
priority: P3
area: chat
status: done
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352039
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
---

## Problem

Two registered chat verbs are effectively unused:

- `knock`: 1 use in 14 days.
- `answer`: 1 use in 14 days.

Both are functionally redundant with verbs that *are* used (`ask`
has 15 uses; `say`/`fyi` cover the answer use case). Carrying them
costs menu-list real estate, the cadence-skill prose that lists
"when to use which", CLI/MCP code paths, and conceptual surface for
new users.

## Context

Removal touches:

- `internal/chat/` — verb registration + dispatch.
- `cmd/squad/knock.go`, `cmd/squad/answer.go` (or whichever files
  define the cobra commands).
- `internal/mcp/` — MCP tool registrations for the verbs.
- `plugin/skills/squad-chat-cadence` — the verb table in the
  skill body.
- `CLAUDE.md` "Chat cadence" table — same list.
- `AGENTS.md` (generated) — regenerate after the verb registry
  changes via `squad scaffold agents-md`.
- The dashboard SPA's chat-verb rendering / kind-color map.

Hard-remove (no soft-deprecation): the dogfood project is the only
user; no external migration concerns.

## Acceptance criteria

- [ ] `squad knock` and `squad answer` exit with usage error
      (cobra "unknown command").
- [ ] MCP tool list no longer exposes `squad_knock` or
      `squad_answer`. Existing MCP clients calling them get a
      tool-not-found error.
- [ ] The chat-cadence skill table contains exactly the remaining
      verbs: `thinking`, `milestone`, `stuck`, `fyi`, `ask`,
      `say`, `handoff`. No mention of knock/answer in any skill.
- [ ] CLAUDE.md "Chat cadence" table matches the skill table.
- [ ] AGENTS.md is regenerated and matches.
- [ ] The dashboard SPA renders nothing for historical knock/
      answer messages already in the ledger besides their stored
      kind (graceful render, no crash on legacy data).
- [ ] A grep of the codebase for `"knock"` and `"answer"` as chat
      kinds returns zero hits in active code paths (historical
      messages in the DB are fine and visible).
- [ ] Existing tests covering the removed verbs are deleted, not
      ported.

## Notes

- Conceptual cleanup. Smaller surface area is the deliverable; no
  user-visible feature removal beyond menu cleanup.
- Sequence: ship before CHORE-022 (skills/CLAUDE.md regenerate
  from live verb registry) and CHORE-023 (stats panel cleanup),
  both of which depend on the final verb list.

## Resolution
(Filled in when status → done.)
