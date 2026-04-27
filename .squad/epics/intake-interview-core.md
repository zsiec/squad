---
spec: intake-interview
status: open
parallelism: |
  Mostly serial within the package. Migration first, then session/turns/status
  on the same data layer, then validation and commit on top. Atomicity tests
  for commit are independent of the wiring tasks.
---

## Goal

Build the `internal/intake` package and migration `009_intake_interview.sql`. By
the end of this epic the package can: open/resume/cancel sessions, append
turns, recompute `still_required`, return status, validate bundles structurally,
detect slug conflicts, and commit either an `item_only` or `spec_epic_items`
bundle atomically — including the refine-mode supersedes flow.

## Scope

- Migration: `intake_sessions`, `intake_turns`, `items.intake_session_id`.
- Embedded checklist YAML + per-repo override loader.
- Session lifecycle (open/resume/cancel/status) with one-open-per-agent invariant.
- Turn append + `still_required` honor-system computation.
- Bundle structural validation (both shapes, refine-mode constraint).
- Slug conflict check across DB and disk.
- Commit pipeline with atomic write + rollback.
- Refine-mode commit: archive original, claim_history row.

## Out of scope (handled in other epics)

- MCP tool wiring (intake-interview-mcp).
- CLI subcommands (intake-interview-cli).
- Slash command, skill, manifest (intake-interview-plugin).
