---
id: FEAT-049
title: scaffold AGENTS.md from current ledger state
type: feature
priority: P2
area: internal/scaffold
status: done
estimate: 3h
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
blocked-by: []
parent_spec: agent-team-management-surface
epic: documentation-contract-generated-agents-md
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

`AGENTS.md` is a hand-edited prose file at the repo root. It is supposed
to tell an agent what is in flight, what is ready, and what was recently
done — but that data lives in the ledger and the prose drifts the moment
the ledger advances. There is no command that regenerates it from the
ledger, so every update is a manual edit racing against reality.

## Context

`squad init` already writes a templated `AGENTS.md` from
`internal/scaffold/`, but the template is static text — it does not query
the DB. The data the file should reflect is already available:

- Ready items via `internal/items/walk.go` plus the in-DB priority order.
- In-flight claims via `internal/claims/`.
- Recent done items via the same item walker filtered on status.
- Active specs and epics via `internal/specs/` and `internal/epics/`.

What is missing is a generator that pulls those four queries together,
renders them into a markdown body, and writes it under a do-not-edit
banner. CLAUDE.md must remain untouched — it is the hand-edited contract.

## Acceptance criteria

- [ ] `squad scaffold agents-md` (or equivalent verb under `squad
  scaffold`) writes `AGENTS.md` from the current ledger state, replacing
  whatever body was there.
- [ ] Output sections include: top 5 ready items (id, title, priority);
  in-flight claims (id, title, claimant, intent); last 10 done items (id,
  title, summary); active specs and epics index with links to the
  underlying markdown files.
- [ ] The generated file opens with a do-not-edit banner reading roughly
  "do not edit by hand; regenerate with squad scaffold agents-md".
- [ ] `CLAUDE.md` is not touched by the command — it remains the only
  hand-edited contract file.
- [ ] A test in `internal/scaffold/` exercises the generator against a
  seeded fixture DB and asserts the rendered sections.

## Notes

- Investigate the current `AGENTS.md` shape before designing the template
  so the generated body covers what readers already rely on.
- Treat the generator as a pure function from ledger snapshot to markdown
  string; the file write should be a thin wrapper so the test can assert
  on the string directly.
- CHORE-015 depends on this item — the pre-commit hook compares the
  current `AGENTS.md` against the generator's output to detect drift.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
