# Hygiene

## Why it exists

Multi-agent state decays. An agent crashes mid-claim; a touch never gets released; an item file goes missing because the agent on the other machine renamed it. Without active maintenance, the global DB and the per-repo `.squad/items/` directory drift apart, and `squad next` starts returning items that someone else is half-done with but not actively working on.

The hygiene sweep is the answer: a periodic, in-process pass that flags drift and (where safe) auto-corrects.

## How it runs

Hygiene runs **as a goroutine inside the next DB-touching command,** debounced via `~/.squad/hygiene.lock` (default: 5–10 seconds between runs). There's no separate daemon. If you never run a squad command, hygiene never runs — and that's fine, because nothing is touching the DB to drift.

The runner:

1. Acquires the lock file (best-effort; multiple processes back off cleanly).
2. Marks agents stale that haven't ticked past the threshold.
3. Reclaims claims whose owner is now stale.
4. Releases the lock.

Errors are silently swallowed by design — hygiene is a maintenance concern, not a critical path. A failed sweep at 09:00:01 just means the next command at 09:00:15 retries.

## What `squad doctor` checks

`squad doctor` is the explicit, on-demand sweep. It surfaces problems without auto-fixing the destructive ones:

- **Stale claims** — agents with no `last_tick_at` past `hygiene.stale_claim_minutes`. Doctor lists them; you pick whether to `squad force-release`.
- **Ghost agents** — registered agents with no recent activity at all. Cosmetic; they age out of `squad who` over time.
- **Orphan touches** — file-touch records whose claim has already closed. Doctor flags them with a suggested `squad untouch <path>` command; release is user-initiated.
- **Broken refs** — claim rows pointing at item IDs that don't exist on disk (file got deleted or renamed in a peer's branch). Doctor lists; you decide.
- **DB integrity** — `PRAGMA integrity_check` against `~/.squad/global.db`. Should always be `ok`; if not, restore from a backup before continuing.

Exit code 0 means clean. Non-zero means at least one issue was reported.

## What an unhealthy state looks like

```
$ squad doctor
stale claims: 1
  BUG-042  agent-blue   last_tick 2h17m ago    → squad force-release BUG-042 --reason "agent-blue gone"
orphan touches: 3
  internal/cache/flusher.go    by agent-blue → squad untouch internal/cache/flusher.go
broken refs: 1
  FEAT-099  no item file in .squad/items/ or .squad/done/
db integrity: ok
exit 1
```

Each problem includes the command to fix it. Run those, then re-run `squad doctor` until it exits 0.

## Manual run timing

- **After a crash** of any session that may have held a claim.
- **Before resuming a stale repo** you haven't worked in for >1 day.
- **Whenever peers say your claim looks stuck** — they're right; tick or release.
- **As a weekly habit** in long-lived multi-agent projects, even when nothing is obviously wrong.

## What the sweep does NOT do

- It doesn't delete item files. Drift between filesystem and DB is reported, never silently resolved by deletion.
- It doesn't compact the DB. SQLite's WAL grows during long-lived processes; if `~/.squad/global.db-wal` gets large, run `sqlite3 ~/.squad/global.db "VACUUM"` manually.
- It doesn't migrate schema. Schema migrations happen at startup of any DB-opening command.
- It doesn't notify anyone. Findings are printed to your terminal — no Slack, no email, no telemetry.
