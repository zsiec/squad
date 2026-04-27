---
id: FEAT-041
title: approved learnings auto-PR CLAUDE.md update
type: feature
priority: P2
area: internal/learning
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

**AC bullet 1 (audit):** refuted. The previous flow ran `git apply`
against the working tree and archived the proposal — no branch, no
commit, no PR. `gh pr create` was never invoked anywhere in the
codebase.

**AC bullet 2 (implementation):** `cmd/squad/learning_agents_md_approve.go`:
- Save the current branch via `git rev-parse --abbrev-ref HEAD`.
- Create + checkout `squad/learning-agents-md-<id>` via `git checkout -B`
  (idempotent on retry — if the branch exists it gets reset to current
  HEAD instead of failing with exit-128).
- `git apply` the diff.
- Stage `AGENTS.md`, commit with `docs(agents-md): apply learning <id>`.
- When `origin` is configured AND `gh` is on PATH: `git push -u origin
  <branch>` then `gh pr create --head <branch>`. Otherwise skip with a
  populated `PRSkippedReason` so the operator can finish manually.
- Restore the original branch via `git reset --hard HEAD` + `git
  checkout <orig>` so a partial `gitApply` cannot strand the working
  tree dirty.
- Archive the proposal under `applied/`.

`runGit` only injects a synthetic `squad@local` identity when the repo
has no `user.email` configured — real users keep authorship on their
PR commits. New helpers: `gitCurrentBranch`, `gitCheckoutNewBranch`,
`gitCheckout`, `gitDeleteBranch`, `gitAddCommit`, `gitHasRemote`
(probes `origin` specifically since that's what `gitPush` targets),
`gitPush`, `ghCreatePR`.

`LearningAgentsMdApproveResult` gains `Branch`, `PRURL`, and
`PRSkippedReason` fields so MCP callers see the structured outcome and
the cobra command surfaces both branch + PR URL (or skip reason).

**AC bullet 3 (manual e2e):** to verify against a real remote,
1. From this repo: `git diff -- AGENTS.md > /tmp/x.diff` (with a
   trivial AGENTS.md tweak staged in your working tree).
2. `squad learning agents-md-suggest --diff /tmp/x.diff --rationale
   "manual e2e"`.
3. Note the proposal id from the printed path.
4. `squad learning agents-md-approve <id>`.
5. Expect stdout to show `branch: squad/learning-agents-md-<id>`,
   `pr: https://github.com/...` (or a PR-skipped reason if running
   in a repo without `origin`/`gh`), and `applied: ...`.
6. The current branch should be back to where it was before the call.

**Tests:**
- `cmd/squad/learning_agents_md_approve_lib_test.go::TestAgentsMdApprove_PureBranchesAndArchives`
  asserts the new contract at the pure-function level (renamed from
  `_PureAppliesAndArchives` since the contract changed: in-place →
  branched).
- `cmd/squad/learning_agents_md_approve_test.go::TestAgentsMdApprove_BranchesAndArchivesWithoutRemote`
  pins the cobra-level no-remote path: branch + commit happen
  locally, stdout calls out the skip, original branch is restored.
- The conflict tests still pin proposal-preserved on apply failure;
  the new `restore` closure (with `git reset --hard`) keeps the
  working tree clean.
