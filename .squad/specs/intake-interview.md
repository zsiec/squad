---
title: Interview-driven intake
motivation: |
  squad's existing intake (squad new + accept/refine/reject) assumes the user
  already knows what they want to file. Anything bigger than a one-liner —
  features, initiatives, refactors — gets stubbed and either rots in the
  captured queue or forces the user to write a spec from scratch in a text
  editor before the work has any structure. There is no first-class flow for
  "here's a rough idea, help me figure out the spec, the epics, and the items."
acceptance:
  - "/squad:squad-intake \"<rough idea>\" opens an interview session and produces a validated bundle"
  - bundle adapts shape — item_only or spec_epic_items — based on what's warranted
  - required-fields checklist is enforced server-side; user "ship it" sign-off is the final gate
  - committed items default to captured, with a --ready opt-in
  - refine mode hydrates an existing captured item and supersedes it on commit
  - transcripts persist in SQLite so sessions are resumable across Claude restarts
  - exactly one open intake session per agent per repo at a time
non_goals:
  - multi-agent collaboration on a single intake session
  - automatic generation of acceptance criteria, dependencies, or scope edges without user validation
  - editing of committed artifacts via intake (use the normal lifecycle)
  - natural-language understanding of turn content for checklist tracking
  - checklist customization via the interview itself (edit .squad/intake-checklist.yaml instead)
  - first-class draft-on-disk staging mode in v1
  - analytics dashboard of intake sessions
integration:
  - internal/store (new migration 009, items.intake_session_id column)
  - internal/intake (new package — sessions, turns, validation, commit)
  - internal/specs and internal/epics (frontmatter gains intake_session field)
  - internal/items (parent_spec, epic_id, intake_session_id back-links)
  - cmd/squad (new intake subcommand tree, four new MCP tool registrations)
  - plugin/squad (squad-intake slash command + skill, manifest version bump)
---

## Background

Companion design and plan documents live at:

- `~/dev/switchframe/docs/plans/2026-04-27-intake-interview-design.md`
- `~/dev/switchframe/docs/plans/2026-04-27-intake-interview.md`

Both gitignored on both ends per CLAUDE.md.

## Decisions (with rationale)

- **State location: hybrid.** Squad persists transcript and metadata; Claude drives questions. Audit + resume without making squad an authoring environment.
- **Output shape: adaptive.** Bundle declares `item_only` or `spec_epic_items`. No forced ceremony for small ideas.
- **Stop rule: hybrid.** Server-enforced required-fields checklist plus user "ship it" sign-off.
- **Surface: both.** CLI is source of truth; slash command thin-wraps it.
- **Post-commit status: captured by default.** `--ready` to skip review queue.
- **Sign-off granularity: one final review.** Print bundle, accept y/n/edit.
- **Refine mode: supported.** `squad intake refine <id>` hydrates an existing captured item.

## Non-goals (verbose)

See the design doc for full rationale on each non-goal. The short version: intake is a focused tool for the rough-idea-to-structured-artifacts moment; everything outside that arc has its own home in the existing surface.
