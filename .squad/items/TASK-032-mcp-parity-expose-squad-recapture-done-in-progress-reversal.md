---
id: TASK-032
title: 'MCP parity: expose squad recapture (done → in-progress reversal)'
type: task
priority: P2
area: mcp
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-1f3f
captured_at: 1777255765
accepted_by: web
accepted_at: 1777255893
references:
  - cmd/squad/recapture.go
  - cmd/squad/mcp_register.go
  - cmd/squad/mcp_schemas.go
relates-to: []
blocked-by: []
---

## Problem

`squad recapture` moves an item from `done/` back to `items/` and flips its status — the reverse of `squad done`. Used in the refinement-round flow; without MCP exposure agents can't run that loop without shelling.

## Context

Cobra constructor: `cmd/squad/recapture.go:18 newRecaptureCmd`. The lifecycle-tool group in `mcp_register.go` is the right home (next to `squad_done` / `squad_release`).

The refinement flow: reviewer's `## Reviewer feedback` is moved into `## Refinement history` as `### Round N — YYYY-MM-DD`, status flips to `captured`, claim is released. That bookkeeping must happen identically in the MCP path.

## Acceptance criteria

- [x] `Recapture(ctx, RecaptureArgs) (*RecaptureResult, error)` in
  `cmd/squad/recapture.go`. Cobra `runRecapture` wraps it.
- [x] `squad_recapture` registered in `cmd/squad/mcp_register.go` lifecycle
  group with `schemaRecapture` in `cmd/squad/mcp_schemas.go`.
- [x] Migration + claim release run atomically — `items.Recapture` does the
  rewrite and the `DELETE FROM claims` inside one `store.WithTxRetry`. Note
  this is the **Refine** transactional pattern (file rewrite inside the
  tx), not the Done compensation pattern; the AC's "Done pattern" wording
  was imprecise. The Refine shape is correct here because
  `MoveFeedbackToHistory` is idempotent on retry.
- [x] Handler tests in `cmd/squad/mcp_test.go`: `TestMCP_RecaptureRoundTrip`
  (happy) and `TestMCP_RecaptureNoClaim` (error). Happy-path also asserts
  the `### Round 1` header is emitted in `## Refinement history`.
- [x] Expected-tools list in `TestMCP_ListsAllTools` includes
  `squad_recapture`.
- [x] `go test ./cmd/squad/ -count=1 -race` passes — see Resolution.

## Notes

Tips parity per BUG-019.

## Resolution

### Fix

`cmd/squad/recapture.go` — extracted `Recapture(ctx, RecaptureArgs)` from
the cobra wrapper. The wrapper now boots its own context and calls the
pure-fn; both CLI and MCP paths go through the same entry. The result
struct carries only the inputs needed for round-trip
(`item_id`, `agent_id`); a `recaptured_at` timestamp was considered but
dropped per code-reviewer feedback (cannot reflect the file timestamp
from inside the wrapper without threading a clock through the items
package, and no caller renders it).

`cmd/squad/mcp_register.go` — registered `squad_recapture` in the
lifecycle group, between `squad_release` and `squad_done`. Handler
shape mirrors `squad_release`: `requireRepo` → `resolveAgentID` →
forward to the pure-fn.

`cmd/squad/mcp_schemas.go` — added `schemaRecapture`.
`{item_id (required), agent_id}` — same shape as `squad_release`.

`cmd/squad/mcp_test.go` — appended two roundtrip tests covering
happy + no-claim, plus `squad_recapture` in the expected-tools list.

### Tips parity

The CLI prints no nudge after `squad recapture` —
`printCadenceNudgeFor` only handles `claim` and `done`. The MCP
handler does not populate `Tips` either, preserving CLI/MCP parity.
No silent-divergence concern.

### Coordination notes

This claim was redone after agent-1f3f's TASK-033 commit absorbed an
earlier in-flight pass via the linter-merge of shared mcp_register/
mcp_schemas files (see their handoff post). The squad_recapture
registration in HEAD is from agent-1f3f's commit; the cobra
extraction, schema, and tests in this commit complete the rest of
the AC.

### Code-reviewer findings addressed

- **AC wording: "Done pattern" → Refine pattern.** Documented in the
  AC checkbox above; no code change needed.
- **`RecapturedAt` field**: dropped (was dead weight).
- **`### Round 1` assertion**: added to the happy-path test so a
  regression that empties the migration but keeps the section header
  fails loudly.

Two reviewer nits not addressed: the no-claim test doesn't exercise
the "wrong agent holds it" branch (only the no-row branch); and the
JSON-RPC error code falls through to `errInternal` (-32603) rather
than `errInvalidParams` (-32602). Both match the existing sibling
lifecycle handlers (`squad_release`, `squad_blocked`). A sweep
across all three would be a separate follow-up.

### Evidence

```
$ go test ./cmd/squad/ -run "TestMCP_Recapture|TestMCP_ListsAllTools" -count=1
ok  	github.com/zsiec/squad/cmd/squad	0.536s

$ go test ./cmd/squad/ -count=1 -race
ok  	github.com/zsiec/squad/cmd/squad	63.185s

$ golangci-lint run
0 issues.
```

Attestation hashes:
- `2e4877f25ff0e81e951cfb18dae2346310ee07c3a174de174d92844772bbdb2a`
- `e2bbef9303f24dbc7b0deceab4408cdcbf2f0be3ec7b6a3e568c217df7933c94`
