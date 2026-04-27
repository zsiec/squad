---
id: BUG-029
title: worktree-by-default scaffold breaks cmd/squad attest+done test fixtures (no main ref)
type: bug
priority: P1
area: cli
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777311164
accepted_by: web
accepted_at: 1777312850
references: []
relates-to: []
blocked-by: []
---

## Problem

CHORE-014 (commit `19ffa82`, "opt squad init into worktree-per-claim by default") flips the scaffold default for `default_worktree_per_claim` to true. After that change, several `cmd/squad` tests that exercise the claim → attest → done lifecycle fail because their git fixtures do not have a `main` branch — the worktree-by-default `squad claim` immediately runs `git worktree add -b squad/<id>-<agent> ... main` and git rejects the unknown ref. The verification gate (`go test ./... -race`) at `squad done` close-time consequently fails for any unrelated work whose author runs the full suite, blocking legitimate close-outs.

Failures observed at HEAD `fd0f29e`:

```
--- FAIL: TestAttest_TestKind_HappyPath (0.07s)
--- FAIL: TestAttest_PositionalItemID
--- FAIL: TestAttest_Review_BlockingFindingsRecordsExit1
--- FAIL: TestAttest_Review_CleanFindingsRecordsExit0
--- FAIL: TestDone_BlockedWhenEvidenceMissing
--- FAIL: TestDone_ProceedsWhenEvidenceSatisfied
--- FAIL: TestDone_Force_RecordsManualAttestation
--- FAIL: TestR4_FullLifecycle
--- FAIL: TestR4_DoctorFlagsForceClosedItemThatLatersLosesAttestation
```

Sample line: `claim --worktree: git worktree add -b squad/FEAT-001-agent-e2a1 .../FEAT-001 main: exit status 128: fatal: invalid reference: main`.

## Context

The fixtures live in `cmd/squad/attest_test.go`, `cmd/squad/done_test.go`, and the R4 lifecycle test in the same package. They `gitInitDir(t, repoDir)` then run squad commands without making a first commit / without aliasing the default branch — that was fine when `--worktree` was opt-in but trips immediately now that the scaffold writes `default_worktree_per_claim: true`.

Two ways to fix:

1. **Test-side**: have the shared `gitInitDir` (or a sibling helper) create an initial commit on `main` so `git worktree add ... main` resolves. This restores the test contract and matches what real users have (a non-empty repo with a default branch).
2. **Code-side**: make `squad claim --worktree` skip the worktree creation gracefully when there is no commit yet (warn-and-fall-back). Less surgical; conflates "no commits" with "user mis-typed the base branch." The test-side fix is preferred.

Both fixes also need `cmd/squad/attest_test.go` and `cmd/squad/done_test.go` to either disable the scaffold default in their setup (set `default_worktree_per_claim: false` in the seeded config) or rely on the helper.

## Acceptance criteria

- [ ] All nine listed failing tests in `cmd/squad/` pass under `go test ./cmd/squad/ -race -count=1` after the fix.
- [ ] `go test ./... -race` is green from a pristine checkout — the squad verification gate stops blocking unrelated `squad done` calls.
- [ ] The fix preserves the spirit of CHORE-014 (worktree-per-claim default stays true in `squad init` scaffolds for real users); it changes test fixtures, not the production default.
- [ ] If the chosen fix is the helper-creates-initial-commit approach, the helper change is documented in a one-line comment on the helper so a future test author knows why the dummy commit exists.
- [ ] No new flakiness: tests run -race and -count=1 cleanly across at least three back-to-back invocations.

## Notes

Discovered while closing FEAT-032 (auto-refine inbox JSON propagation). Logged the failures at HEAD `fd0f29e` after stashing my own diff and rerunning the failing tests on pristine code — confirms the regression is in the committed code, not in my work-in-flight.

Peer agent-bbf6 wrote CHORE-014 and noted in their handoff that "full-suite verification skipped because peer's in-flight BUG-028 left internal/intake/commit_run.go:104 referencing undefined captureRefineHistory in the shared workspace" — that prior intake build break masked the worktree regression so the cmd/squad failures never surfaced during CHORE-014 review. The intake build break has since been fixed (commit `880747a`), uncovering this secondary regression.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
