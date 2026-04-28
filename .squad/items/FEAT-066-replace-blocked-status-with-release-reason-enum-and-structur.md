---
id: FEAT-066
title: replace blocked status with release --reason enum and structured note
type: feature
priority: P2
area: claims
status: open
estimate: 4h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777351998
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
auto_refined_at: 1777351962
auto_refined_by: claude
---

## Problem

The `blocked` status path is dead infrastructure: 0 `squad blocked` events in the last 7 days, 0 in 14 days, across 870 release/done events. Operators don't escalate to blocked — they call `squad release` with prose like *"still blocked on FEAT-014 schema; the system keeps re-handing me this since blocked-by field is empty"* or *"blocked: needs FEAT-014 schema; held by agent-401f"*. The real signal exists; it's just stuffed into a free-form `outcome` field that has become an unstructured dumping ground.

## Context

Two coupled changes:

1. Drop the `blocked` state branch. `squad blocked <ID>` flips an item to `status: blocked` and the state machine has affordances around unblock, doctor sweeps, and ready-stack filtering — all of which fire zero times in observed practice.

2. Replace the free-form `outcome` field on release with a structured enum:
   - `released` — intentional handoff, work continues
   - `superseded` — claim moot (e.g. another item subsumed it)
   - `blocked` — gating dependency on another item
   - `abandoned` — gave up, no follow-up planned

   Optional `--note` carries the prose. The release write path and the SPA's release modal both need to surface the enum.

The auto-postmortem trigger (FEAT-061) currently distinguishes release/blocked paths by status; after this lands, it triggers off `reason in (blocked, abandoned)` instead, simpler.

## Acceptance criteria

- [ ] `squad release <ID> --reason <released|superseded|blocked|abandoned>` is the new contract; the flag is required and missing it errors with a usage line listing the four values.
- [ ] `squad release <ID> --reason <X> --note "<prose>"` accepts optional free-form prose, persisted alongside the enum value in the claims store.
- [ ] `squad blocked <ID>` is removed end-to-end: the CLI command, the `status: blocked` state transition, the doctor sweep over blocked items, and the ready-stack filter all drop together.
- [ ] A one-time idempotent data migration classifies existing free-form `outcome` strings into the new enum via case-insensitive `contains` for "block", "supersede", "abandon", defaulting to `released`.
- [ ] The auto-postmortem trigger fires when `reason IN ('blocked', 'abandoned')` AND the existing artifact-presence detector still says "no durable artifact"; it does not fire on `released` or `superseded`.
- [ ] The dashboard SPA's release modal renders the four-way reason picker plus an optional note field, and the chosen reason is sent on submit.
- [ ] Documentation is updated: `CLAUDE.md`, `AGENTS.md`, `docs/README.md`, `docs/adopting.md`, and any docs/recipes/troubleshooting pages that mention `squad blocked` or free-form `outcome` are revised to describe the new `--reason` contract.
- [ ] Tests: migration verified against a fixture DB with mixed-prose outcomes; `release --reason` end-to-end tested per enum value; auto-postmortem test fires only on `blocked`/`abandoned`; CLI test asserts `squad blocked` is gone (command not found).

## Notes

- Coordinated cleanup with CHORE-022 (regenerate skills/CLAUDE.md from live verb registry) and CHORE-023 (stats panel tile cleanup) — those depend on this landing first.
- Risk: text-pattern migration is best-effort. Acceptable since the existing `outcome` data is operator-shorthand; perfect classification isn't required.

## Resolution
(Filled in when status → done.)
