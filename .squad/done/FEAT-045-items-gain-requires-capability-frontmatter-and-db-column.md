---
id: FEAT-045
title: items gain requires_capability frontmatter and DB column
type: feature
priority: P2
area: internal/store
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309558
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: capability-routing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Items have no way to declare which agent capabilities are required to
claim them. The ready stack is a flat priority queue — every agent sees
every ready item regardless of fit. Capability routing cannot start
without a typed slot on the item itself.

## Context

This is the schema foundation for the capability-routing epic. Three
moving pieces have to land together so the rest of the epic has
something to read:

- **Migration.** `internal/store/migrations/` currently tops out at
  `009_intake_interview.sql`. Add `010_items_requires_capability.sql`
  that runs `ALTER TABLE items ADD COLUMN requires_capability TEXT NOT
  NULL DEFAULT '[]'`. The default makes existing rows valid without a
  data backfill.
- **Marker probe.** `internal/store/migrate.go:bootstrapLegacyVersions`
  records markers for migrations that ran on databases predating the
  versions table. The CHORE-009 work just added v5–v8 markers; follow
  the same pattern to probe for the new column and stamp v10 if
  present.
- **Item struct + parser.** `internal/items/item.go` defines the YAML
  frontmatter struct. Add `RequiresCapability []string` with the
  matching YAML tag. The parser must default to an empty slice when
  the key is absent so existing item files round-trip unchanged.

The column stores a JSON-encoded string array (e.g. `["go","sql"]`) for
consistency with how other list-shaped fields are persisted. No index
is needed at this stage — the filter in FEAT-047 reads the agent's set
and intersects in Go, not SQL.

## Acceptance criteria

- [ ] New migration `010_items_requires_capability.sql` adds
  `items.requires_capability TEXT NOT NULL DEFAULT '[]'`.
- [ ] `items.Item` gains a `RequiresCapability []string` field; the
  YAML parser handles the new key and defaults to `[]` when absent.
- [ ] `bootstrapLegacyVersions` gains a marker probe for the new column
  that stamps the migration version when the column is already
  present.
- [ ] Existing item files (no `requires_capability` key) parse with an
  empty slice and remain claimable by any agent.
- [ ] Migration test covers fresh DB and pre-existing-column paths.

## Notes

Scope is items only. The agents table does not gain a capability column
in this item — FEAT-046 owns whatever agent-side persistence is needed
for `squad register --capability`. Keeping the two schema changes in
separate items keeps each migration small and reviewable.

## Resolution

- `internal/store/migrations/010_items_requires_capability.sql`:
  `ALTER TABLE items ADD COLUMN requires_capability TEXT NOT NULL DEFAULT '[]'`.
  The default makes existing rows valid without a backfill.
- `internal/store/migrate.go`: `bootstrapLegacyVersions` gains a v10 marker
  probe following the v5–v9 pattern — stamps the version when the column is
  already present so the migration doesn't try to ALTER it twice.
- `internal/items/items.go`: `Item` gains `RequiresCapability []string` with
  yaml tag `requires_capability` (snake_case matching the modern field
  neighborhood — `evidence_required`, `intake_session_id`, etc.). Absent
  key parses as nil slice; tests pin the empty-slice case.
- `internal/items/persist.go`: `persistUpsert` threads
  `requires_capability` through INSERT and DO UPDATE SET; uses the same
  json.Marshal + nil-coercion-to-`"[]"` pattern as `conflicts_with` so the
  column is always valid JSON for downstream `json_each` use. Out of strict
  AC scope but the item's Notes section explicitly anchors FEAT-047 on
  reading from the column — leaving it out would have meant the column
  always reads `[]` regardless of frontmatter.
- Tests: `TestParse_RequiresCapability` (inline list parse),
  `TestParse_RequiresCapabilityAbsentDefaultsEmpty` (legacy round-trip),
  `TestMigrate_AppliesItemsRequiresCapability` (fresh-DB migration creates
  the column with the right shape and stamps v10), and
  `TestMigrate_BootstrapStampsRequiresCapabilityV10` (pre-existing-column
  legacy bootstrap stamps v10 even when migration_versions got dropped).
  Extended `TestPersist_PreservesR3Fields` to round-trip the field through
  SQL — caught a class of bug where someone could later add the column
  to the INSERT but forget the DO UPDATE SET clause (or vice versa).
- Updated three pre-existing version-count assertions from 9→10
  (`TestMigrate_BootstrapsLegacyDBWithoutIntakeColumns`,
  `TestMigrate_BootstrapPreservesWorktreeAndSeedsAllVersions`,
  `TestMigrate_IntakeInterviewIdempotent_From008`). Reviewer noted this
  is a pattern that will repeat at v11/v12 — worth parameterizing later;
  three call sites isn't enough yet.
