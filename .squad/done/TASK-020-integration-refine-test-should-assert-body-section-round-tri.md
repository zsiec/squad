---
id: TASK-020
title: integration_refine_test should assert body-section round-trip (Reviewer feedback → Refinement history)
type: task
priority: P3
area: server
status: done
estimate: 15m
risk: low
created: 2026-04-26
updated: "2026-04-27"
captured_by: agent-1f3f
captured_at: 1777246940
accepted_by: web
accepted_at: 1777247099
references:
  - internal/server/integration_refine_test.go
relates-to: []
blocked-by: []
---

## Problem

`TestIntegration_RefineRoundTrip` exercises the full refine → list → claim → recapture → inbox loop but only asserts status flips and inbox membership. It does NOT assert that:

- After `/refine`, the file body contains a `## Reviewer feedback` section with the comments verbatim.
- After `/recapture`, that section has moved into `## Refinement history` as a `### Round N` entry, and is no longer present as `## Reviewer feedback`.

The body-section round-trip IS tested at the unit level (`TestRewriteWithFeedback_RoundTrip`, `TestRewriteRecapture_RoundTrip`, `TestItemsRecapture_AppendsRound2`), but the cross-cutting integration test is the right place to lock the behavior end-to-end.

## Context

`internal/server/integration_refine_test.go`. After each POST in the test, read the item file from disk and `strings.Contains` the expected sections. Two-line patch each.

Discovered during inbox-refinement epic final review.

## Acceptance criteria

- [ ] After step 2 (POST `/refine`), assert the on-disk body contains `## Reviewer feedback` with the comments string.
- [ ] After step 5 (POST `/recapture`), assert the on-disk body contains `## Refinement history` with `### Round 1`, and does NOT contain `## Reviewer feedback`.
- [ ] Test still passes; output pasted in the close-out chat.

## Notes

Polish, not a correctness gap — the unit tests already prove the body rewrite. Filed to keep the integration test honest as a living spec.

## Resolution

### Fix

`internal/server/integration_refine_test.go`:
- After step 1 (POST `/refine`): read the on-disk file and assert `## Reviewer feedback` and the literal comment text both appear.
- After step 4 (POST `/recapture`): read the file again and assert `## Refinement history` and `### Round 1` are present, and `## Reviewer feedback` is gone (moved into history).

### Evidence

```
$ go test ./internal/server -run TestIntegration_RefineRoundTrip -v -count=1
=== RUN   TestIntegration_RefineRoundTrip
--- PASS: TestIntegration_RefineRoundTrip (0.01s)
PASS
ok  	github.com/zsiec/squad/internal/server  0.376s
```

Full `go test ./... -count=1 -race` passes (0 FAIL lines).

### AC verification

- [x] Post-refine: asserts `## Reviewer feedback` + comment text.
- [x] Post-recapture: asserts `## Refinement history` + `### Round 1` present, `## Reviewer feedback` absent.
- [x] Test still passes.
