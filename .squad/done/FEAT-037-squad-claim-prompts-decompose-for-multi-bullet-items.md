---
id: FEAT-037
title: squad claim prompts decompose for multi-bullet items
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
accepted_at: 1777309557
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: refinement-and-contract-hardening
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Items with many acceptance bullets that span distinct files map cleanly to
multiple PR-sized units of work, but `squad claim` accepts them as a single
claim with no signal that decomposition would help. The agent picks up the
item, ships the first one or two bullets, and either balloons the PR to
cover everything or releases mid-way with the rest deferred. `squad
decompose` exists for exactly this case — we just don't surface it at the
moment of decision.

## Context

`cmd/squad/claim.go` already has the nudge pattern in place: after a
successful claim, `printSecondOpinionNudge` (called at line 211) prints a
peer-ask suggestion when priority is `P0`/`P1` or risk is `high`, and
`printMilestoneTargetNudge` (line 212) suggests milestone cadence based on
`items.CountAC(parsed.Body)`. A new nudge follows the same shape: parse
the claimed item, run the heuristic, print one informational line on
stderr if it fires.

Heuristic: count the acceptance-criteria checkboxes
(`items.CountAC` already exists in `internal/items/counts.go`) and count
distinct file references inside the bullets. A "file reference" is any
token in the bullet matching a path-shaped pattern — at minimum
`<path>/<file>.<ext>` or a Go package path like `internal/foo/bar`. If
both the bullet count is at least 4 AND the distinct file-reference count
is at least 3, print the nudge.

The nudge is informational only. The claim still succeeds. Decomposition
is a recommendation, not a precondition — sometimes a 4-bullet item
genuinely is one PR.

## Acceptance criteria

- [ ] After a successful claim in `cmd/squad/claim.go`, the parsed item's
      AC bullets are scanned. When `len(acceptance) >= 4` AND
      `distinct_file_references >= 3`, a one-line nudge is printed to
      stderr suggesting `squad decompose <ID>` before continuing.
- [ ] The file-reference count is implemented in `internal/items/` (likely
      a new helper alongside `counts.go`) so it can be unit-tested without
      booting the cobra command.
- [ ] The nudge does NOT block the claim — exit code stays 0, the claim
      row is inserted, the existing `printCadenceNudge` /
      `printSecondOpinionNudge` / `printMilestoneTargetNudge` lines all
      still print as they do today.
- [ ] Test in `cmd/squad/claim_lib_test.go` (or a new file in
      `internal/items/`) covers a fixture body that should trigger the
      nudge and one that should not.

## Notes

Use the existing `printSecondOpinionNudge` as the template for the new
function — same signature shape (writer + parsed fields), same single-line
output to stderr, same lowercase prefix style. Naming suggestion:
`printDecomposeNudge`.

The path-shape regex needs to be permissive enough to catch
`internal/intake/commit_run.go`, `cmd/squad/claim.go`, and bare
`AGENTS.md` — but tight enough not to count `e.g.` or `i.e.` as paths.
A reasonable starting point: a token containing at least one `/` and
ending in `.<ext>` OR a token of two or more slash-separated segments
where each segment matches `[a-zA-Z0-9_-]+`. Iterate against the existing
items in `.squad/items/` if needed.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
