---
id: BUG-019
title: MCP claim/done handlers skip stderr nudges — agents using MCP tools never see them
type: bug
priority: P1
area: cli
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777250300
accepted_by: agent-bbf6
accepted_at: 1777250368
references:
  - cmd/squad/mcp_register.go
  - cmd/squad/claim.go
  - cmd/squad/done.go
  - cmd/squad/cadence_nudge.go
relates-to:
  - TASK-015
  - TASK-016
  - TASK-017
blocked-by: []
---

## Problem

The three claim-time nudges (`printCadenceNudge`, `printSecondOpinionNudge`, `printMilestoneTargetNudge`) and the type-aware done-time nudge (`printCadenceNudgeFor`) all live in the cobra `RunE` wrappers in `cmd/squad/claim.go` and `cmd/squad/done.go`. The MCP-side claim and done handlers in `cmd/squad/mcp_register.go` call the underlying `Claim()` / `Done()` library functions *directly* and return their structured results as JSON, bypassing the wrappers entirely. Result: agents that interact via MCP (most of the dogfood session for `feature-uptake-nudges`, including the orchestrator agent-bbf6 until the explicit CLI walkthrough at TASK-019) never see any of the new nudges. The epic's measured uplift is therefore a floor, not a ceiling — every MCP-using agent is unreached.

## Reproduction

1. Build the binary fresh:

```bash
go build -o /tmp/squad-uptake ./cmd/squad
```

2. Pick (or create) a P0/P1/risk:high item with ≥2 AC. From the CLI:

```bash
$ /tmp/squad-uptake claim TASK-019 --intent "demo"
claimed TASK-019
  tip: `squad thinking <msg>` to share intent · silence with SQUAD_NO_CADENCE_NUDGES=1
  tip: high-stakes claim — consider `squad ask @<peer> "sanity-check my approach?"` before starting · silence with SQUAD_NO_CADENCE_NUDGES=1
  tip: 14 AC items — expect ~14 'squad milestone' posts as you green each one · silence with SQUAD_NO_CADENCE_NUDGES=1
```

Three nudges fire on stderr, as designed.

3. From an MCP client (or via `mcp__plugin_squad_squad__squad_claim`), claim the same item:

```json
{"item_id":"TASK-019","agent_id":"agent-bbf6","intent":"demo","claimed_at":1777246315}
```

Zero nudges. The result payload is just the `ClaimResult` struct serialized to JSON — no `tips`, `nudges`, or any equivalent surface for the client to render.

Same pattern for `squad_done`: the CLI prints the type-aware learning nudge after success; the MCP path returns the `DoneResult` payload without the nudge text.

## Context

- **Cobra wrapper for claim** at `cmd/squad/claim.go:156-165` calls `printCadenceNudge`, `printSecondOpinionNudge`, `printMilestoneTargetNudge` after the `Claim()` library call returns success. Each helper writes one stderr line.
- **MCP claim handler** at `cmd/squad/mcp_register.go:139-172` skips this entirely:

```go
return Claim(ctx, ClaimArgs{...})
```

- **Cobra wrapper for done** at `cmd/squad/done.go:182` calls `printCadenceNudgeFor(cmd.ErrOrStderr(), "done", itemType)`. The MCP done handler (same file as above) calls `Done()` and returns its result.

The fix has to surface the nudge text in the MCP response payload — agents reading tool results see them as part of the JSON, not as out-of-band stderr (which MCP clients typically discard).

## Fix sketch (recommended shape; reviewer may push back)

1. **Add text-returning helpers alongside the existing `print*` ones in `cmd/squad/cadence_nudge.go`.** Each returns `""` when silent (env suppressed, type doesn't match, AC count below threshold, etc.). Existing helpers become thin `Fprintln` wrappers around the text variants — DRY.

```go
func cadenceNudgeText(kind, itemType string) string { ... }
func secondOpinionNudgeText(priority, risk string) string { ... }
func milestoneTargetNudgeText(acTotal int) string { ... }
```

2. **Extend `ClaimResult` and `DoneResult` with an optional `Tips []string` field (`json:"tips,omitempty"`).** The CLI doesn't populate it (it still writes stderr directly via `print*`). The MCP handler populates it from the text-returning helpers after the library call succeeds.

3. **Wire in `mcp_register.go`:** after `Claim()` succeeds, read the item via `findItemPath`+`items.Parse`, gather the three nudges into `Tips`, attach to the result. Same shape for `Done()` with the single learning nudge.

4. **Honor `SQUAD_NO_CADENCE_NUDGES`** the same way as the CLI path — the helpers already check it; the text variants should too.

5. **Tests:** unit tests for the text-returning helpers (parallel to existing `print*` tests), plus an MCP-handler test asserting that `tips` is populated on a P1+multi-AC claim and absent on a P3+single-AC claim.

## Acceptance criteria

- [ ] `cadenceNudgeText`, `secondOpinionNudgeText`, `milestoneTargetNudgeText` exist in `cmd/squad/cadence_nudge.go` returning the same strings the existing `print*` helpers write (sans trailing newline). Empty string when silent / suppressed.
- [ ] Existing `print*` helpers are reimplemented as thin wrappers around the text variants — no duplicated copy.
- [ ] `ClaimResult` and `DoneResult` gain `Tips []string \`json:"tips,omitempty"\`` field.
- [ ] MCP claim handler (`cmd/squad/mcp_register.go`) populates `Tips` after `Claim()` succeeds, reading priority/risk/AC count from the parsed item.
- [ ] MCP done handler populates `Tips` with the type-aware learning nudge after `Done()` succeeds (read item type from the moved file).
- [ ] Both handlers honor `SQUAD_NO_CADENCE_NUDGES=1` (return `Tips` as `nil`/omitted when set).
- [ ] Unit tests for each `*Text` helper (mirroring the existing `Test*Nudge` tests) covering all the same matrix points but asserting on returned-string content.
- [ ] MCP-handler test asserts `tips` field is populated correctly on a P1 multi-AC item and omitted on a P3 single-AC item.
- [ ] CLI behavior is byte-identical (the print wrappers must not regress — assert via the existing CLI tests).
- [ ] `go test ./...` passes; trailing summary pasted into close-out chat.

## Notes

- **Discovered during dogfood walkthrough** (TASK-019, 2026-04-27). Surfaced as one of three follow-up findings in the epic-completion milestone.
- Severity P1 because the entire `feature-uptake-nudges` epic (8 items, ~3 hours of work) is currently unreached for MCP-using agents. That's "shipped a product nobody can see" territory.
- Don't change MCP tool *schemas* (input shape is unchanged). Only the *response* shape gains an optional field, which is back-compatible — existing clients that ignore unknown fields keep working.
- The chat-cadence skill should not need updating; the user-visible behavior is the same content, just delivered through a different surface.
- If a future MCP client wants to suppress tips entirely, `SQUAD_NO_CADENCE_NUDGES=1` already works; no per-call flag needed.

## Resolution

(Filled in when status → done.)
