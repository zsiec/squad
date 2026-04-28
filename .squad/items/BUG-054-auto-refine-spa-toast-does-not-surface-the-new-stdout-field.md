---
id: BUG-054
title: auto-refine spa toast does not surface the new stdout field
type: bug
priority: P3
area: spa
status: captured
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: 2026-04-28
captured_by: agent-401f
captured_at: 1777342934
accepted_by: ""
accepted_at: 0
references: []
relates-to: []
blocked-by: []
---

## Problem

The auto-refine error response now carries both `stdout` and `stderr`
(server-side fix already shipped), but the SPA's toast renderer in
`internal/server/web/inbox.js:285` reads only `stderr` on the 502 path
and neither field on the 500 "no-draft" path. When `claude -p` exits
non-zero with empty stderr — the most common failure mode, since
`claude` writes auth failures, MCP init errors, and tool denials to
stdout — the operator's dashboard toast still shows the bare error
message ("exit status 1") with no diagnostic. The new server-side data
is reachable in DevTools but invisible in the UI that surfaced the
original report.

## Context

`internal/server/web/inbox.js` 274-296:

- 502 case reads `payload.stderr` only, falls back to `payload.error`.
- 500 case shows `payload.error` ("claude exited without drafting; run
  again") with neither stdout nor stderr.

The server contract is established by the just-shipped change to
`internal/server/items_auto_refine.go`. The two new server tests
(`TestAutoRefine_NonZeroExitIncludesBothStreams`,
`TestAutoRefine_NoWriteIncludesBothStreams`) document the wire shape;
the SPA needs to consume what's now on the wire.

## Acceptance criteria

- [ ] On a 502 auto-refine response with non-empty `stdout` and empty
      `stderr`, the toast body shows the stdout snippet (truncated to
      ~240 chars) instead of the bare error message.
- [ ] On a 500 "no-draft" auto-refine response, the toast body shows
      the available diagnostic — preferring `stdout` when present, else
      `stderr`, else the error message.
- [ ] Structural Go test reading the embedded SPA bytes pins that
      `inbox.js` references both `stdout` and `stderr` payload fields
      in the auto-refine toast switch (same `webFS.ReadFile` pattern as
      `repo_badge_css_test.go`'s `TestRepoBadgeCssIsDistinctlyStyled`).

## Notes

Surfaced during code review of the server-side fix. Reviewer's
recommendation was to scope-split rather than fold into the same item
since the AC there was strictly server-side. The toast renderer is
small — the change is roughly two lines per case, plus a structural
test.

## Resolution
(Filled in when status → done.)
