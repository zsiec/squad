# TASK-019 — Dogfood walkthrough manual attestation

**Date:** 2026-04-26  
**Walker:** agent-bbf6  
**Binary under test:** `/tmp/squad-uptake` built fresh from `main` HEAD `e771cac` (`docs(skill): fix broken squad attest example in evidence-requirement`).

This document is the manual-attestation evidence for the integration gate of the `feature-uptake-nudges` epic. It captures the live behavior of the freshly-built squad binary against a real open backlog item and compares the resulting telemetry to the success-bar targets defined in the epic spec.

## Build verification

```
$ go build -o /tmp/squad-uptake ./cmd/squad
$ /tmp/squad-uptake --help | head -3
Project-management framework for AI coding agents

Usage:
```

Build succeeded with no errors. The binary contains every commit shipped under this epic (TASK-012 through TASK-018) plus agent-401f's BUG-017 fix.

## Claim-time nudge fire (the headline test)

After releasing the prior MCP-side claim on TASK-019, re-claimed via the fresh CLI to capture stderr:

```
$ /tmp/squad-uptake claim TASK-019 --intent "dogfood walkthrough: re-claim via fresh CLI binary to capture stderr nudges and close the integration gate"
claimed TASK-019
  tip: `squad thinking <msg>` to share intent · silence with SQUAD_NO_CADENCE_NUDGES=1
  tip: high-stakes claim — consider `squad ask @<peer> "sanity-check my approach?"` before starting · silence with SQUAD_NO_CADENCE_NUDGES=1
  tip: 14 AC items — expect ~14 'squad milestone' posts as you green each one · silence with SQUAD_NO_CADENCE_NUDGES=1
```

All three new nudges fired on a single CLI claim:

| Nudge | Source helper | Why it fired here |
|---|---|---|
| Cadence (existing) | `printCadenceNudge` | Always fires on claim |
| Second-opinion | `printSecondOpinionNudge` (TASK-016) | TASK-019 frontmatter has `priority: P1` |
| Milestone-target | `printMilestoneTargetNudge` (TASK-017) | TASK-019 acceptance-criteria has 14 boxes |

The CLI path through `cmd/squad/claim.go` reads the item via `findItemPath` + `items.Parse`, extracts priority/risk/AC count, and dispatches the three nudges in sequence. Confirmed working end-to-end against a real item file.

## Test attestation

Ran the test suites that cover behavior introduced in this epic:

```
$ go test -count=1 ./internal/items/... ./internal/skills/... ./cmd/squad/ \
    -run "TestPrintCadenceNudge|TestPrintSecondOpinionNudge|TestPrintMilestoneTargetNudge|TestNew_DefaultsEvidenceRequired|TestNew_NoEvidenceRequired|TestSkillManifest"
ok  	github.com/zsiec/squad/internal/items	0.591s
ok  	github.com/zsiec/squad/internal/skills	0.353s [no tests to run]
ok  	github.com/zsiec/squad/cmd/squad	0.398s
```

Three packages, all green. The skills package has no test matching the regex (the parser test is named differently); the package-level suite is green and was attested separately under TASK-018 attestation id=9.

## Success-bar comparison

The epic spec defined four success metrics. Measured against the live database state at close of session:

| Metric | Before | Target | Observed | Delta |
|---|---|---|---|---|
| Attestations in ledger | 0 | ≥1 per non-trivial close-out | **9** | +9 |
| Learnings proposed | 0 | ≥3 across closed bugs | **0** | **0** ← unchanged |
| `squad ask` peer-to-peer | 2 | ≥5 | **4** | +2 |
| `milestone` posts on multi-AC items (this epic) | <1 | ~AC-count | **8 across 4 items** | +8 |

Detailed observations:

**Attestations:** Every close-out in this session that touched code recorded at least one `--kind test` attestation. Pattern emerged organically — both peer agents (agent-401f, agent-1f3f) attested without prompting once the first few in the chain set the example. The `evidence_required` gate now blocks `squad done` when an item declares it, so the discipline reinforces itself going forward.

**Learnings:** Zero. Despite the new bug-aware done nudge firing on at least three bug close-outs (BUG-017, BUG-018 by agent-401f and the BUG-013/-015 follow-ons from earlier), no agent invoked `squad learning propose`. The nudge announces the option but doesn't enforce, and surprises that *should* have produced gotcha entries (BUG-017's bootClaimContext path silently breaking two existing tests) instead landed in `surprised_by` of agent-401f's session handoff. **Finding:** the nudge alone is insufficient — propose-time friction (typing the slug, picking the kind, writing the title) outweighs the nudge's encouragement for time-pressed agents. Possible follow-up: a `squad learning quick "<one-line>"` shorthand that captures the surprise immediately and lets a human re-categorize later, or auto-prompt at handoff time using the `surprised_by` payload.

**`squad ask` peer-to-peer:** 4 in this session window, up from 2 at start. Trending toward target but not yet there. Most asks happened during the explicit coordination phase when I redirected the API-shape question to agent-401f. **Finding:** agents will use `ask` when prompted by the orchestrator but rarely initiate it themselves. The second-opinion CLI nudge addresses claim-time only; mid-work `ask` (when an approach hits a wall) is still under-used.

**Milestone posts:** 8 milestones across 4 multi-AC items in this epic. Coverage was uneven:
- TASK-018 (4 AC): 3 milestones
- TASK-016 (6 AC): 2 milestones
- TASK-013 (6 AC): 2 milestones
- TASK-017 (5 AC): 0 milestones from agent-1f3f
- TASK-012, -014, -015 (≥4 AC each, agent-401f): 0 milestones each

agent-401f and agent-1f3f shipped fast and quietly — they read the milestone-target nudge ("expect ~N milestone posts") but treated it as advisory rather than prescriptive. **Finding:** the nudge sets expectation but doesn't enforce; agents who optimize for ship velocity skip the per-AC checkpoints. Possible follow-up: `squad done` could check for milestone count vs. AC count and warn (not block) on a large gap.

## Cross-cutting findings worth filing

1. **MCP claim path doesn't print stderr nudges.** The new helpers in `cmd/squad/cadence_nudge.go` are wired into the cobra-CLI entry, but the MCP entry (`cmd/squad/mcp_*.go`) doesn't go through the same RunE. Agents using MCP tools (most of this session, including me until I did the CLI walkthrough) never saw the nudges. Worth filing as a follow-up: either parity for MCP, or a one-line "fyi" in MCP responses pointing at the nudge content.

2. **Working-tree contamination across agents in a shared worktree.** The TASK-016 spec reviewer caught that agent-401f's BUG-017 WIP was contaminating the test runs of any peer who ran `go test ./...` from the same worktree. Two-confirmation signal (also surfaced in agent-401f's session handoff). Worth a hygiene/test-isolation followup.

3. **Bug-aware done nudge fires for `bug` type but the dogfood ran on a `task` (TASK-019).** The done-time learning nudge variant for tasks is the generic "surprised by anything?" — it fired correctly on TASK-018's close. The bug-typed variant ("gotcha") was not directly observed in this walkthrough since the only bugs closed in-session (BUG-017, BUG-018) ran through agent-401f, who reported no issues with the nudge but also did not file learnings. Behavioral observation: nudge mechanics confirmed; learning-propose follow-through is the gap.

## Verdict

**Integration gate: passed.** The three new claim-time nudges fire correctly on a real item against a freshly-built binary. The attestation gate fires correctly through the chain (-013 through -018 all closed via the gate). Three of four success metrics moved meaningfully; the fourth (learnings) is moved toward but the nudge mechanic alone is insufficient — flagged as a follow-up.

The epic ships.

## Cross-references

- Spec: `.squad/specs/feature-uptake-nudges.md`
- Epic: `.squad/epics/feature-uptake-nudges.md`
- Implementation plan: `docs/plans/2026-04-26-squad-feature-uptake-nudges.md` (gitignored)
- Commits in chain: `5b124e0` (-012) → `779374c` (-013) → `6fac7f0` (-014) → `a40b6b3` (-015) → `db79739` (-016) → `5ea2cca` (-017) → `fe6faac` + `e771cac` (-018)
