---
id: FEAT-050
title: auto-rendered specs and epics index page under docs
type: feature
priority: P2
area: internal/scaffold
status: open
estimate: 2h
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

Specs and epics live as markdown files under `.squad/specs/` and
`.squad/epics/`, but there is no index page. A reader who wants to know
"what specs are active, and which epics belong to each" has to `ls` the
directories and open files one by one. The data to render an index is
already in the ledger; the index itself just does not exist.

## Context

`internal/specs/` and `internal/epics/` already expose listings and
relationships. The directories `.squad/specs/` and `.squad/epics/` hold
the underlying markdown — those files are the link targets the index
should point at.

The auto-refine peer's recent FEAT-029 through FEAT-032 set is the
clearest current example of a multi-item epic: it shows what an entry in
the epics index needs to display (status, item count, link to the epic's
markdown body).

This item is the second leg of the documentation-contract epic. It pairs
with FEAT-049 (generated `AGENTS.md`): together they cover the index of
what is happening (`AGENTS.md`) and the index of what is planned
(`docs/specs.md`, `docs/epics.md`).

## Acceptance criteria

- [ ] `squad scaffold doc-index` writes `docs/specs.md` and
  `docs/epics.md` from current ledger state.
- [ ] Each entry in `docs/specs.md` links to the underlying spec
  markdown file under `.squad/specs/`; each entry in `docs/epics.md`
  links to its file under `.squad/epics/`.
- [ ] Each index entry shows the spec's or epic's status (active, done,
  cancelled) and, for epics, the count of items belonging to it.
- [ ] A test in `internal/scaffold/` renders both files against a seeded
  fixture and asserts the link targets, status labels, and item counts.

## Notes

- Keep the rendering pure-function-style for the same reason as FEAT-049:
  test against the string, not the file.
- The two files should be regenerable in isolation — running the command
  twice with no ledger changes should produce byte-identical output.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
