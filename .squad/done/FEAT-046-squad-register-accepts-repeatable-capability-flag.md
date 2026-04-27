---
id: FEAT-046
title: squad register accepts repeatable --capability flag
type: feature
priority: P2
area: cmd/squad
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309559
references: []
relates-to: []
blocked-by: [FEAT-045]
parent_spec: agent-team-management-surface
epic: capability-routing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Agents have no way to declare what they can do. Without a per-agent
capability set, the ready-stack filter in FEAT-047 has nothing to
intersect against — every agent looks identical to the router.

## Context

`cmd/squad/register.go` defines the existing register command and
writes the agent row through `internal/identity`. The agents table
(`internal/store/migrations/001_initial.sql`) has columns for id,
repo, display name, worktree, pid, timestamps, and status — no
capability column today.

FEAT-045 adds the items-side column only. This item owns the
agent-side schema change as well: a follow-up migration that adds
`agents.capabilities TEXT NOT NULL DEFAULT '[]'`, plus the CLI flag
plumbing and `whoami` rendering. Bundling them keeps the agent-side
story in one item; splitting the migration off would create a
half-finished CLI flag with nowhere to write.

Set semantics: re-registering with a new `--capability` set replaces
the prior set. Append-on-re-register would silently accumulate stale
tags and is the wrong default — registration is the agent declaring
its current shape, not a history of every shape it has ever had.

## Acceptance criteria

- [ ] Migration `011_agents_capabilities.sql` adds
  `agents.capabilities TEXT NOT NULL DEFAULT '[]'` with the matching
  marker probe in `bootstrapLegacyVersions`.
- [ ] `squad register --capability go --capability sql` persists
  `["go","sql"]` on the agent row.
- [ ] Re-running `squad register --capability frontend` replaces the
  set; the agent row reads `["frontend"]`, not `["go","sql","frontend"]`.
- [ ] `squad whoami` renders the registered capability set (empty set
  prints as `(none)` or equivalent).
- [ ] Test covers register-then-re-register and the empty-set case.

## Notes

Tag values are free-form strings, lowercased on input. No central
registry of allowed tags — the operator chooses their own taxonomy
(`go`, `sql`, `frontend`, `design`, `planning`, etc.). FEAT-047's
intersection logic treats unknown tags as opaque, which keeps the
feature usable without a config step.

## Resolution

- `internal/store/migrations/011_agents_capabilities.sql`: `ALTER TABLE
  agents ADD COLUMN capabilities TEXT NOT NULL DEFAULT '[]'`. Existing
  rows stay valid without a backfill.
- `internal/store/migrate.go`: `bootstrapLegacyVersions` gains a v11
  marker probe that stamps the version when `agents.capabilities` is
  already present.
- `cmd/squad/register.go`: `RegisterArgs` gains `Capabilities []string`
  + `SetCapabilities bool` sentinel. New `normalizeCapabilities` helper
  lowercases / trims / dedupes / sorts so re-register input order
  doesn't drift the column. `upsertAgent` only writes `capabilities`
  when `SetCapabilities` is true — `squad go`'s implicit re-register
  every session must NOT silently wipe what the operator declared via
  `squad register --capability`. Cobra `--capability` is repeatable
  (StringSliceVar) and the flag flips `SetCapabilities` via
  `cmd.Flags().Changed("capability")`. Empty set under
  `SetCapabilities=true` (or `--capability ""`) is the explicit reset.
- `cmd/squad/whoami.go`: `WhoamiResult.Capabilities []string` with
  `omitempty`; reads JSON from `agents.capabilities`. New `--verbose`
  text mode prints `id:` and `capabilities:` lines (`(none)` for an
  empty set); default text output stays id-only so existing scripts
  aren't broken; `--json` includes the array when populated.
- `cmd/squad/mcp_register.go` + `mcp_schemas.go`: MCP `squad_register`
  tool now declares the `capabilities` array property and forwards it
  to `RegisterArgs`. Unset → preserve; explicit `[]` → clear; populated
  → replace. Without this, MCP register would silently wipe the column
  on every implicit re-register.
- `cmd/squad/go.go`: `ensureRegistered` passes `setCaps=false` so the
  session-boot re-register preserves the prior set.
- Tests:
  - `register_lib_test.go`: `TestRegister_PersistsCapabilities`,
    `TestRegister_ReregisterReplacesCapabilities`,
    `TestRegister_EmptyCapabilitiesPersistsAsEmptyArray`,
    `TestRegister_LowercasesAndDedupesCapabilities`, plus
    `TestRegister_ReregisterWithoutCapabilityFlagPreservesPriorSet`
    (the regression test for the silent-wipe blocker the reviewer
    caught — confirms an implicit re-register without the flag keeps
    the prior column).
  - `whoami_test.go`: `TestWhoami_RendersCapabilities` (--json
    includes the array), `TestWhoami_EmptyCapabilitiesShowsNone`
    (--verbose renders `(none)`).
  - `migrate_test.go`: `TestMigrate_AppliesAgentsCapabilities`
    (fresh-DB column shape + v11 stamp),
    `TestMigrate_BootstrapStampsAgentsCapabilitiesV11` (legacy
    bootstrap stamps v11 when column already present). Updated three
    pre-existing version-count assertions from 10→11.
