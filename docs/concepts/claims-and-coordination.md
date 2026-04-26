# Claims and coordination

## What a claim is

A claim is an atomic, exclusive lock on one item by one agent on one machine. The claim record lives in `~/.squad/global.db` (SQLite) and is acquired via `BEGIN IMMEDIATE` — if two agents race for the same item, exactly one wins and the other gets a clean "already claimed by agent-XXXX" error. Released claims move to `claim_history` so a peer can see who held what when.

A claim is a CLI command, not a frontmatter field. The item file's `status:` is rewritten at close-out (when `squad done` runs), not at claim time.

## Why files + DB hybrid

Item content is **per-repo, durable, git-committed.** It lives in `.squad/items/<ID>-<slug>.md` and travels with the codebase. That's where the AC is, where the body is, where the rollback plan is.

Claims, chat, and file touches are **machine-local, operational, ephemeral.** They live in `~/.squad/global.db` and are recreated on the next register. Trying to merge claim rows in git would be a nightmare — two agents claim simultaneously on different machines, both commits "win," and now you have a phantom claim that no one holds. The hybrid keeps git diffs about behavior changes and the DB about who's doing what right now.

## Multi-agent rules

- **One claim per agent per repo** by default. The configuration knob (`agent.claim_concurrency` in `.squad/config.yaml`) can lift it, but the default is 1 because a single agent juggling multiple claims is usually thrashing.
- **Heartbeat keeps a claim live.** Every `squad tick`, `squad milestone`, `squad thinking`, etc. updates the `last_touch` timestamp on your active claim. A claim with no activity past the configured `hygiene.stale_claim_minutes` (default 60 min) is flagged by `squad doctor` as stale.
- **`squad force-release <ID>`** takes over a stuck claim from a peer who isn't responding. The command requires `--reason` so the audit trail in `claim_history` records why the takeover happened. Good citizenship: post `squad ask @agent-XXXX "stealing X, ok?"` first if the peer is reachable.
- **File-touch tracking** warns (does not block) when you start editing a file that another agent's claim is touching. `squad touch <path>` declares an active touch; `squad untouch <path>` releases it. The opt-in pre-edit hook automates this against Edit/Write tool calls.

## Lifecycle states

```
filed (.squad/items/) ──┬─► claimed ──► in-progress ──► review ──► done (.squad/done/)
                        │       │              │
                        └─► blocked            └─► released (back to ready)
                                │
                                └─► (resolved) ──► claimed
```

Command per transition:

| From | To | Command |
|---|---|---|
| filed | claimed | `squad claim <ID> --intent "..."` |
| claimed | released | `squad release <ID>` |
| claimed | review | `squad review-request <ID>` |
| any open | blocked | `squad blocked <ID> --reason "..."` |
| blocked | claimed | `squad claim <ID>` (re-claim) |
| any open | done | `squad done <ID> --summary "..."` |

`squad reassign <ID> @new-owner` is shorthand for "release + ping new owner in chat."

## Cross-machine claims

The global DB is **machine-local**. If you switch laptops, your claim does not follow you — register on the new machine, then re-claim. Items themselves do follow (they're in git), so the work is portable; only the operational state needs re-creating.

Cross-machine claim sync is **out of scope for v1.** v2 may add it (the design doc has a section on this). For now: one machine = one claim namespace.

## Common races and how they resolve

- **Two agents claim simultaneously.** SQLite `BEGIN IMMEDIATE` serializes the transactions; one commits, the other gets `unique_constraint`-equivalent and the `squad claim` command exits with a clear "already claimed by X" message. No corruption, no torn state.
- **Agent crashes mid-claim.** No release runs, so the claim stays open with a stale heartbeat. The next `squad doctor` run flags it; a peer can `squad force-release` after confirming.
- **Claim across worktrees in the same repo.** Each worktree's `.squad/items/` may differ if the items are in different branches, but the DB is shared. Claiming the same `<ID>` from two worktrees still races against the DB, so only one wins — even if the other worktree doesn't have that file checked out.

## See also

- [squad-vs-agent-teams.md](squad-vs-agent-teams.md) — claim semantics compared to agent-teams' file-locked tasks.
