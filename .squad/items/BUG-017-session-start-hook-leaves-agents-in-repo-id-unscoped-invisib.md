---
id: BUG-017
title: session_start hook leaves agents in repo_id=_unscoped — invisible to squad who
type: bug
priority: P2
area: cli
status: open
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777246479
accepted_by: web
accepted_at: 1777246535
evidence_required: [test]
references:
  - plugin/hooks/session_start.sh
  - cmd/squad/register.go
  - internal/identity/
relates-to:
  - BUG-002
blocked-by: []
---

## Problem

When a Claude Code session starts inside a squad-managed repo, the SessionStart hook registers the agent — but the registration row is filed under `repo_id=_unscoped`. Subsequent `squad who` (which scopes by current repo_id) doesn't see the agent, even though the agent is actively claiming and posting in the repo. Manual re-registration via `squad register --as <id>` from inside the repo upgrades the row to the correct repo_id; without that step, the agent remains invisible.

## Reproduction

1. Open a fresh Claude Code session in `/Users/zsiec/dev/squad`. Confirm the SessionStart hook fires (it prints `[squad] registered as agent-XXXX in squad`).
2. From that session, post a chat message: `squad fyi "test"` — succeeds and updates `last_tick_at`.
3. From that session, claim an item: `squad claim <ID> --intent "..."` — succeeds; the claim is correctly scoped to the squad repo.
4. Run `squad who` — the agent is **not** in the output.
5. Inspect the DB directly:

```bash
sqlite3 ~/.squad/global.db "SELECT id, repo_id, status FROM agents WHERE id = 'agent-XXXX'"
```

Returns the row with `repo_id = _unscoped` despite the claim and chat being correctly scoped.

6. Re-register from inside the repo: `squad register --as agent-XXXX --name agent-XXXX`. The row now has `repo_id = <actual repo id>`, and `squad who` shows the agent.

## Context

This is the same shape as BUG-002, which was fixed for the `squad go` upgrade path. The hook-driven path (`plugin/hooks/session_start.sh`) appears to call `squad register --no-repo-check` (or whatever the equivalent is) and never re-runs the upgrade once it's clear which repo the session is in. Agents that ran `squad go` after the hook were already covered by the BUG-002 fix; agents that drove the squad lifecycle from MCP/Claude tools and never invoked `squad go` stayed in `_unscoped`.

The chat and claim paths both correctly resolve the repo_id at the call site, which is why posting/claiming works fine even with the wrong agents-table row. The dashboard and `squad who` are the visible failure surfaces.

## Acceptance criteria

- [ ] RED test: a fresh agent registered with `--no-repo-check` (the hook path), followed by any repo-scoped action (claim, chat, tick), upgrades the agents-table row's `repo_id` from `_unscoped` to the resolved repo id.
- [ ] Audit and pick the right upgrade trigger: probably either inside `Claim()` (most reliable since it's the first repo-scoped write the agent makes) or a dedicated "upgrade if needed" check in the MCP request prelude. Document the choice in the resolution.
- [ ] After the fix, a fresh session that registers via the hook and then claims an item is visible to `squad who` without manual `squad register` re-runs.
- [ ] Existing `_unscoped` agents-table rows from before the fix are not orphaned: pick a migration strategy (sweep on first repo-scoped op, or stale-clean after `hygiene.stale_claim_minutes`). Document in resolution.
- [ ] `go test ./...` passes; trailing `ok` line pasted into close-out chat.

## Notes

- Confirmed live during epic feature-uptake-nudges work — agent-bbf6 was missing from `squad who` despite holding a claim on TASK-013 and having posted multiple messages. Manual re-register fixed it.
- Don't conflate with BUG-002. BUG-002 fixed the `squad go` path's agentExists check ignoring repo_id; the lingering issue is the hook path that never reaches `squad go`.
- Lower priority than the feature-uptake epic — file and defer until that closes.

## Resolution

(Filled in when status → done.)
