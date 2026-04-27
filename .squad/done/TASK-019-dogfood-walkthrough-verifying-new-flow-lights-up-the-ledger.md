---
id: TASK-019
title: dogfood walkthrough verifying new flow lights up the ledger
type: task
priority: P1
area: dogfood
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777245998
accepted_by: agent-bbf6
accepted_at: 1777245998
epic: feature-uptake-nudges
evidence_required: [manual]
references: []
relates-to: []
blocked-by:
  - TASK-012
  - TASK-013
  - TASK-014
  - TASK-015
  - TASK-016
  - TASK-017
  - TASK-018
---

## Problem

The whole epic is justified by an empirical claim — that lighting up the gate plus three nudges will make the attestation, learning, and peer-chat features start producing data. We need to demonstrate that on at least one real open item before declaring the epic done.

## Context

By the time this item runs, every code-side change in the epic has landed. The current open backlog (BUG-014, CHORE-002, EXAMPLE-001, TASK-008/-010/-011) gives plenty of options for a real-item walkthrough — pick whichever is closest to ready and not currently claimed by a peer.

## Acceptance criteria

- [ ] Build the binary fresh from `main` so all epic changes are in: `go build -o /tmp/squad-uptake ./cmd/squad && /tmp/squad-uptake --help` succeeds.
- [ ] Pick one open item not currently claimed by another agent. Run `squad claim <ID>` and observe:
  - [ ] Cadence nudge fires (`squad thinking <msg>` tip).
  - [ ] Second-opinion nudge fires *if* item is P0/P1 or risk:high (otherwise silent — note observed priority/risk in close-out).
  - [ ] Per-AC milestone-target nudge fires *if* item has ≥2 AC boxes.
- [ ] Work the item per the loop. Post `squad milestone` at each AC. Post at least one `squad thinking` mid-flow.
- [ ] Capture test output to a file and run `squad attest <ID> --kind test --command "go test ./..." --output <file>`.
- [ ] Run `squad done <ID> --summary "..."`. The done call must succeed without `--force`. The bug-aware done nudge must fire (with `gotcha` if the item is a bug).
- [ ] If the close-out surfaced any non-obvious learning, file it via `squad learning propose ...`.
- [ ] Verification queries:
  - [ ] `sqlite3 ~/.squad/global.db "SELECT COUNT(*) FROM attestations WHERE item_id = '<ID>'"` returns ≥1.
  - [ ] `squad learning list` shows any newly-proposed entries.
  - [ ] `sqlite3 ~/.squad/global.db "SELECT COUNT(*) FROM messages WHERE thread = '<ID>' AND kind = 'milestone'"` returns ≥AC-count for multi-AC items.
- [ ] Close-out note: paste the exact terminal output of each `squad claim`, `squad attest`, `squad done`, and the three verification queries into the resolution section so future readers can compare expected vs. observed behavior.

## Notes

- `evidence_required: [manual]` is deliberate — this is a manual-verification item; recording the walkthrough output as a `--kind manual` attestation closes the loop on the gate this whole epic just turned on.
- If any nudge or gate misbehaves, file a follow-up BUG immediately rather than fixing inline — the dogfood is the test, and a passing dogfood with a known small bug filed is a more honest signal than a dogfood the executor patched on the fly.
- This is the integration gate for the whole epic. If it passes cleanly, the epic ships. If not, the failed step is a follow-up item.

## Resolution

(Filled in when status → done.)
