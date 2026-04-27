---
id: FEAT-041
title: approved learnings auto-PR CLAUDE.md update
type: feature
priority: P2
area: internal/learning
status: open
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
epic: observation-to-knowledge-pipeline
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

The MCP tools `squad_learning_agents_md_suggest`,
`squad_learning_agents_md_approve`, and `squad_learning_agents_md_reject`
exist by name. Their naming implies they produce a CLAUDE.md edit,
but it is unclear whether the approve path actually opens a PR or
just flips a row in the suggestions table. If it is the latter, the
end-to-end loop the rest of this epic is trying to close terminates
in a database, not in a merged file.

## Context

Search for the tool names in `cmd/squad/` and `internal/learning/`.
The expected wiring is: `suggest` proposes a CLAUDE.md edit, `approve`
renders the edit as a diff and invokes `gh pr create` against the
target repo, `reject` records the dismissal with rationale.

If the wiring is missing, this item ships it: a diff renderer that
takes the approved suggestion and the current CLAUDE.md, produces a
unified diff, branches the repo, applies the patch, commits, and
opens the PR. `gh` is already an assumed dependency for other PR
flows in the codebase — reuse the same invocation pattern.

## Acceptance criteria

- [ ] Audit confirms (or refutes) that the existing
      `squad_learning_agents_md_approve` writes a PR-ready diff and
      invokes `gh pr create`.
- [ ] If the audit refutes: implement the diff renderer and the
      `gh pr create` invocation so approval lands a real PR.
- [ ] Manual end-to-end test: propose a learning, approve it, observe
      a PR open against the dogfood repo with the CLAUDE.md edit.

## Notes

Step 1 is the audit — do not assume the wiring is missing until you
have read the approve path. The gap may be smaller than the names
suggest.

If a PR already opens, but the diff is wrong (e.g. appends instead of
inserting at the right anchor), file the diff-quality fix as a
separate item — this one is about closing the loop, not perfecting
the edit.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
