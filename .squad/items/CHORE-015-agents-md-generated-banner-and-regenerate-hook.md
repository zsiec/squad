---
id: CHORE-015
title: AGENTS.md generated banner and regenerate hook
type: chore
priority: P3
area: plugin/hooks
status: open
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309559
references: []
relates-to: []
blocked-by: [FEAT-049]
parent_spec: agent-team-management-surface
epic: documentation-contract-generated-agents-md
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Once `AGENTS.md` is generated (FEAT-049), nothing prevents a contributor
from hand-editing it anyway, or from committing a stale version. Without
a guard, the file drifts back into the same prose-vs-ledger gap the epic
is trying to close. The contract needs to be enforced, not just stated.

## Context

`plugin/hooks/` already contains pre-commit hooks the plugin installs
into a managed repo. `pre_commit_pm_traces.sh` is the closest analog: it
inspects staged content and refuses commits that violate a project rule.
Hooks are registered in `plugin/hooks/hooks.json`.

This chore depends on FEAT-049 because the hook needs the generator's
output to compare against. Without the generator, "is this file up to
date" is an unanswerable question.

`CLAUDE.md` is explicitly out of scope — it is the hand-edited contract
and the hook must not touch it.

## Acceptance criteria

- [ ] After `squad scaffold agents-md` (FEAT-049) runs, the generated
  `AGENTS.md` opens with a clear do-not-edit banner.
- [ ] A pre-commit hook under `plugin/hooks/` checks that `AGENTS.md`
  matches the current generator output and refuses commits that
  hand-edit the body.
- [ ] The hook is registered in `plugin/hooks/hooks.json` alongside the
  existing pre-commit entries.
- [ ] `CLAUDE.md` is unaffected by the hook — hand-edits to `CLAUDE.md`
  pass through normally.

## Notes

- Model the hook on `pre_commit_pm_traces.sh` — same shape, same exit
  semantics, same error message style.
- The hook can shell out to the squad binary to regenerate into a temp
  buffer and diff against the staged file, rather than reimplementing
  the generator.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
