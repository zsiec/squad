---
id: FEAT-068
title: pre-commit hook flags pm traces in commit subjects
type: feature
priority: P3
area: hooks
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352040
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
---

## Problem

CLAUDE.md's "No PM traces anywhere in code" rule is mostly held —
out of 231 commits in the last 30 days, 42 had item-IDs in the
subject. About 90% of those are squad's own automated bookkeeping
commits (`chore(squad): close FEAT-XXX`, `chore(squad): refine
CHORE-009 and CHORE-010`) which are the *correct* shape for the
ledger. The remaining ~4 commits are real convention violations
that slipped past the convention. A pre-commit guard closes the
gap with one rule.

## Context

The hook lives in `plugin/hooks/` (PreToolUse hooks already exist
for touch-tracking). A new `pre_commit_pm_trace_check.sh` (or
inline check) examines the staged commit subject when a `git
commit` invocation is detected.

Allowlist: subject prefix `chore(squad):` (case-sensitive). The
squad CLI's auto-commits use this prefix exactly; preserving them
through the gate is the entire point of the carve-out.

Block list: any commit subject containing `BUG-NNN`, `FEAT-NNN`,
`CHORE-NNN`, `TASK-NNN`, or `DEBT-NNN` after the conventional
prefix (`feat:`, `fix:`, etc.) is rejected with a one-line message
pointing at the convention.

## Acceptance criteria

- [ ] `git commit -m "fix: BUG-100 something"` fails locally with
      a message naming the convention and pointing at CLAUDE.md.
- [ ] `git commit -m "chore(squad): close FEAT-100"` succeeds —
      the squad-itself bookkeeping path is allowlisted.
- [ ] `git commit -m "fix: thing referencing FEAT-100 in body
      only"` (item-ID only in body, not subject) succeeds.
      Subject-only check.
- [ ] The hook is registered through the normal scaffold
      mechanism (i.e. `squad init` on a fresh repo produces it).
      Existing repos pick it up via `squad init --update` or a
      one-line CLAUDE.md note pointing at the manual install.
- [ ] A test exercises both the block-path (rejected commit) and
      the allowlist-path (accepted squad bookkeeping commit).

## Notes

- Pre-commit, not post-commit, so the violation never hits the
  history. Reversible: bypass with `--no-verify` if an operator
  genuinely needs to (and accepts the convention violation).
- Companion to the ongoing "agents keep wanting to put item IDs
  in commits" friction. The hook is the enforcement layer for
  the rule that's already in CLAUDE.md.

## Resolution
(Filled in when status → done.)
