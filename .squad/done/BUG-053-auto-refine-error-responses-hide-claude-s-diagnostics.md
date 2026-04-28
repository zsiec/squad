---
id: BUG-053
title: auto-refine error responses hide claude's diagnostics
type: bug
priority: P2
area: server
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777342474
accepted_by: agent-401f
accepted_at: 1777342499
references: []
relates-to: []
blocked-by: []
---

## Problem

When `POST /api/items/{id}/auto-refine` fails because the spawned `claude
-p` exited non-zero, the handler returns `{error, stderr}` and drops
stdout. When it succeeds (exit 0) but didn't advance `auto_refined_at`,
the handler returns `{error, stdout}` and drops stderr. Symmetric blind
spots: in practice the operator sees `{error: "exit status 1", stderr:
""}` or `{error: "claude exited without drafting", stdout: "..."}` with
no way to know what claude actually said, because `claude -p` writes
most of its diagnostic messages — auth failures, MCP init errors, tool
denials — to stdout, not stderr.

## Context

`internal/server/items_auto_refine.go:182-200`:

- The 502 path emits `{error, stderr}` only.
- The 500 "no draft" path emits `{error, stdout}` only.

The runner already captures both streams (`autoRefineRunResult`). The
truncation helper `autoRefineTruncate(b, 512)` is already in place.
Existing tests:

- `TestAutoRefine_NonZeroExitReturns502` / `..._TruncatesStderr` only
  assert stderr.
- `TestAutoRefine_NoWriteIncludesStdoutTail` / `..._TruncatesStdout`
  only assert stdout.

The SPA renders whatever fields are present; widening the response is
backwards-compatible.

## Acceptance criteria

- [x] 502 (non-zero exit) response body contains BOTH `stdout` and
      `stderr` fields, each truncated to 512 bytes.
- [x] 500 (no-draft) response body contains BOTH `stdout` and `stderr`
      fields, each truncated to 512 bytes.
- [x] Truncation cap is symmetric: a 10 KB stream returns exactly 512
      bytes in either field on either path.
- [x] Existing tests asserting only one of the two fields are extended
      (or new tests added) so the missing-direction is pinned.

## Notes

Discovered while debugging an `exit status 1` failure on the live
dashboard — the operator had no way to see why claude failed because
`stdout` was hidden on the 502 path. Symmetric fix is the cheapest path:
log everything we have on both error responses.

## Resolution

`internal/server/items_auto_refine.go` lines 182-202 — the 502
(non-zero exit) and 500 (no-draft) response bodies now both carry
`stdout` and `stderr`, each passed through `autoRefineTruncate(b,
512)`. The runner already captured both streams; the only change was
including the missing direction on each error path.

Tests added in `internal/server/items_auto_refine_test.go`:

- `TestAutoRefine_NonZeroExitIncludesBothStreams` — runner returns
  distinct sentinel strings on stdout and stderr; asserts both surface
  in the 502 body.
- `TestAutoRefine_NoWriteIncludesBothStreams` — same shape against the
  500 path.

Existing single-direction tests (`...IncludesStderr`,
`...IncludesStdoutTail`, plus the symmetric truncation tests) were
left untouched — they pin the original-direction contract and the new
tests pin the simultaneous-presence contract; together they catch any
regression that drops either field on either path. Truncation in the
new directions reuses `autoRefineTruncate(b, 512)` (covered by
existing tests on the original directions).

Code review caught a related SPA-side gap: the toast renderer in
`internal/server/web/inbox.js:285` reads `stderr` only on 502 and
neither field on 500, so the new server-side data is reachable in
DevTools but not visible in the dashboard toast that surfaced the
original report. Filed as a follow-up SPA item (`auto-refine spa
toast does not surface the new stdout field`) — out of scope for
this server-only AC.

Verification:

- `go test ./... -race -count=1` — every package `ok`.
- `golangci-lint run` — `0 issues.`
- New tests fail RED on unfixed code (verified by reviewer reverting
  the 4-line impl change and re-running).
