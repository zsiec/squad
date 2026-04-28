---
id: CHORE-022
title: regenerate skills and claude md from live verb registry
type: chore
priority: P3
area: docs
status: open
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352083
references: []
relates-to: []
blocked-by: [CHORE-020, FEAT-066]
epic: polish-and-prune-from-usage-data
---

## Problem

The "Chat cadence" verb table is hand-maintained in three places:
`CLAUDE.md`, `AGENTS.md`, and the `squad-chat-cadence` skill body.
Once CHORE-020 removes `knock` / `answer` and FEAT-066 removes
`squad blocked`, the three copies will drift unless updated in
lockstep. Today there's no single source of truth — the verb
registry lives in code (`internal/chat/`), but the prose tables
are hand-typed.

## Context

The fix is to *generate* the cadence table from the live verb
registry on `squad scaffold` runs, similar to how AGENTS.md is
already regenerated from the ledger.

- `internal/chat/` exposes the verb list at runtime; promote the
  list to a package-level export (or a JSON file in the binary).
- `cmd/squad/scaffold.go` (or sibling) renders the table into
  the cadence skill on demand.
- CLAUDE.md gets a markered block (similar to the existing
  "managed by squad" section) inside which the table is
  rewritten on scaffold.

## Acceptance criteria

- [ ] The cadence verb table in `plugin/skills/squad-chat-cadence`
      is rewritten by `squad scaffold` from the runtime verb
      list, not maintained by hand.
- [ ] `CLAUDE.md` "Chat cadence" section is bracketed by managed
      markers and rewritten by the same scaffold pass.
- [ ] `AGENTS.md` regeneration picks up the same source list.
- [ ] After CHORE-020 + FEAT-066 land and this scaffold runs,
      none of the three artifacts mention `knock`, `answer`, or
      `squad blocked`.
- [ ] Test: a fixture verb list with one extra verb produces an
      output table containing that verb in all three artifacts.

## Notes

- Blocked by CHORE-020 (verb removal) and FEAT-066 (blocked-
  status removal); only useful once the registry is final.
- Reduces three drift surfaces to one.

## Resolution
(Filled in when status → done.)
