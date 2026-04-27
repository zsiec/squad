---
id: BUG-017
title: session_start hook leaves agents in repo_id=_unscoped — invisible to squad who
type: bug
priority: P2
area: cli
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-27"
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

### Fix

`cmd/squad/boot.go` — `bootClaimContext` now calls a new `upgradeUnscopedAgent(db, agentID, repoID)` helper after resolving repoID. The helper does `UPDATE agents SET repo_id = ? WHERE id = ? AND repo_id = '_unscoped'` — best-effort, swallows errors (boundary path; never blocks the actual operation).

`bootClaimContext` is the universal CLI prelude — every claim/done/release/say/etc. goes through it, AND `postRunHygiene` calls it as part of its sweep. So the upgrade fires opportunistically on the first repo-scoped operation an agent runs after the hook registered them, with no need to thread the call through 30+ subcommands. The implicit migration for pre-existing `_unscoped` rows is the same hygiene post-run hitting bootClaimContext on the next squad command in the repo — no explicit migration script.

### Why bootClaimContext (per AC #2)

Considered Claim() specifically (the AC's first suggestion) and the MCP request prelude (the AC's second). bootClaimContext wins because:
- Single insertion point that covers ALL repo-scoped commands, not just claim.
- Fires from the post-run hygiene hook too, so any agent that ran *any* squad command at any point gets upgraded automatically.
- An MCP-only injection would miss the CLI-only paths (and vice versa).

### Tests

`cmd/squad/boot_unscoped_upgrade_test.go` (new) — `TestBootClaimContext_UpgradesUnscopedAgent`: register --no-repo-check leaves the row at `_unscoped`; calling `bootClaimContext` upgrades it. Sets `SQUAD_NO_HYGIENE=1` so the cobra post-run hygiene hook doesn't drive the upgrade ahead of the assertion (the test would otherwise pass vacuously).

`cmd/squad/go_test.go` and `cmd/squad/register_test.go` — existing tests `TestGoCmd_UpgradesUnscopedAgentFromHookRegistration` and `TestRegister_NoRepoCheck_WritesAgentRow` got `SQUAD_NO_HYGIENE=1`. Both assert intermediate state ("row is still `_unscoped` after register") which would otherwise be defeated by the hygiene post-run upgrading via bootClaimContext.

### Surfacing of test contamination across parallel agents

While iterating, agent-1f3f independently surfaced that the BUG-017 WIP in this worktree was making `TestRegister_NoRepoCheck_WritesAgentRow` fail in their parallel session. Two confirmations from different sessions = real systemic finding. The SQUAD_NO_HYGIENE setenv on the existing tests resolves it; longer-term the `~/.squad/global.db` shared-state pattern across goroutines is worth a dedicated follow-up.

### Evidence

```
$ go test ./... -count=1 -race
... (0 FAIL lines)
```

### AC verification

- [x] RED test: `TestBootClaimContext_UpgradesUnscopedAgent` exercises the no-repo-check → repo-scoped-action upgrade chain.
- [x] Upgrade trigger: bootClaimContext (universal CLI prelude). Documented above.
- [x] After fix, an agent registered via the hook becomes visible to `squad who` after any squad command runs in the repo.
- [x] Pre-existing `_unscoped` rows: implicit migration via the same hygiene post-run path.
- [x] `go test ./...` passes (race-enabled).
