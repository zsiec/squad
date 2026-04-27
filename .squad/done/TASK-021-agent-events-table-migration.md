---
id: TASK-021
title: agent_events table + migration
type: task
priority: P1
area: store
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777251702
accepted_by: agent-bbf6
accepted_at: 1777251702
epic: agent-activity-stream
references:
  - internal/store/migrations/
  - internal/store/schema.sql
relates-to: []
blocked-by: []
---

## Problem

There is no durable storage for tool-call events fired by Claude Code hooks. The hook scripts run, do their side-effect work, and exit — nothing is queryable afterward. To build the per-agent activity stream the spec describes, we need a STRICT-mode SQLite table that the recorder CLI writes into and the server reads from.

## Context

Migrations live under `internal/store/migrations/` (verify count — likely up to `006_*.sql` after `005_claims_pk_fix`; agent-1f3f's FEAT-005 may have added `007_*` for worktree-per-claim, so this might be `008_*`). Run `ls internal/store/migrations/` first to find the next number. The migration runner is in `internal/store/migrate.go`.

## Acceptance criteria

- [ ] New migration file at `internal/store/migrations/<NNN>_agent_events.sql` (next available number) creating:
  ```sql
  CREATE TABLE agent_events (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id      TEXT NOT NULL,
    agent_id     TEXT NOT NULL,
    session_id   TEXT NOT NULL DEFAULT '',
    ts           INTEGER NOT NULL,
    event_kind   TEXT NOT NULL,    -- pre_tool / post_tool / subagent_start / subagent_stop
    tool         TEXT NOT NULL DEFAULT '',
    target       TEXT NOT NULL DEFAULT '',
    exit_code    INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0
  ) STRICT;
  CREATE INDEX idx_agent_events_agent_ts ON agent_events(repo_id, agent_id, ts);
  CREATE INDEX idx_agent_events_repo_ts  ON agent_events(repo_id, ts);
  ```
- [ ] Migration is idempotent on re-apply (the runner already enforces `migration_versions` tracking; this item just needs the SQL to be valid).
- [ ] Add a unit test in `internal/store/migrate_test.go` (or add a new `agent_events_test.go`) that runs the migration against a fresh DB and asserts the table exists with the expected columns and indexes.
- [ ] `go test ./internal/store/...` passes; trailing `ok` line pasted into close-out chat.

## Notes

- STRICT mode is mandatory per repo convention — every other squad table is STRICT.
- No foreign keys: agents can be GC'd before their events; we don't want cascade deletes wiping audit trail.
- `session_id` defaults to empty string because not every hook context has a session id. The recorder CLI passes whatever the env exposes (e.g. `$SQUAD_SESSION_ID`).
- Schema is the foundation; do not pre-populate, do not add validation triggers. Validation lives in the recorder CLI (TASK-022).
- Coordinate with agent-1f3f (FEAT-005, worktree-per-claim) on migration numbering — they're on `007_*` per their handoff. This item gets the next available.

## Resolution

(Filled in when status → done.)
