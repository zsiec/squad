---
id: TASK-029
title: dogfood walkthrough verifying agent activity stream lights up live
type: task
priority: P1
area: dogfood
status: open
estimate: 1h
risk: low
evidence_required: [manual]
created: 2026-04-27
updated: 2026-04-27
captured_by: agent-bbf6
captured_at: 1777251712
accepted_by: agent-bbf6
accepted_at: 1777251712
epic: agent-activity-stream
references: []
relates-to: []
blocked-by:
  - TASK-021
  - TASK-022
  - TASK-023
  - TASK-024
  - TASK-025
  - TASK-026
  - TASK-027
  - TASK-028
  - CHORE-003
---

## Problem

The whole epic is justified by an empirical claim: that recording tool-call events and rendering them in a click-through SPA drawer makes the operator's view of agent activity meaningfully better. We need to demonstrate that on a real running session before declaring the epic done.

## Context

By this item, every code-side change in the epic has landed: schema, recorder, hook wire-ups, server endpoints, SPA drawer + timeline + filters + SSE, redaction config. Build the binary fresh, start `squad serve`, open the SPA, click a live agent, watch events stream.

## Acceptance criteria

- [ ] Build the binary fresh from `main`: `go build -o /tmp/squad-uptake ./cmd/squad`. Confirm the migration runs on first boot (DB schema includes `agent_events`).
- [ ] Start the SPA: `/tmp/squad-uptake serve`. Open in a browser. Confirm the AGENTS panel renders.
- [ ] Click an active agent. Drawer opens. Confirm:
  - Header shows agent id, current claim, last tick
  - Loading spinner appears, then timeline renders
  - Filter chips visible above the timeline (chat / claim / commit / attestation / pre_tool / post_tool / subagent_start / subagent_stop)
  - `Read` events hidden by default (toggle off); other kinds visible
- [ ] In a separate terminal, do real work as the agent being watched:
  - Run a Bash command — within ~1s, a `pre_tool` + `post_tool` event for `Bash` should appear in the open drawer
  - Edit a file — same for `Edit`
  - Post `squad fyi "test"` — chat verb should appear inline alongside the tool events
  - Run `squad attest <ID> --kind test --command "..."` — attestation should appear with badge + command
- [ ] Toggle off `pre_tool`. Verify all `pre_tool` events disappear from the rendered list. Refresh the page; reopen the drawer. Verify `pre_tool` is still hidden (localStorage worked).
- [ ] Test the redaction config:
  - Set `SQUAD_REDACT_REGEX='secret|password'`
  - Run `squad event record --kind pre_tool --tool Bash --target 'echo password=hunter2'` (or trigger via a real Bash with that shape)
  - Verify the resulting drawer entry shows `<redacted>` not `password=hunter2`
- [ ] Test the volume sanity: with `SQUAD_EVENTS_FILTER_READ=1`, run a 5-minute session involving heavy Read calls. Verify `agent_events` row count grows by ≤500 (vs ~2000+ without the filter). Document the actual numbers.
- [ ] Close the drawer. Verify in DevTools that the SSE connection terminates.
- [ ] Document the walkthrough as `.squad/attestations/task029-stream-walkthrough.md` capturing every step's observed output (paste actual command output, not paraphrase). Run `squad attest TASK-029 --kind manual --command "cat <walkthrough_file>"`.

## Notes

- `evidence_required: [manual]` — this is a walkthrough item, not a code item. The walkthrough doc is the manual attestation evidence.
- If any step fails, file a follow-up BUG immediately rather than fixing inline. The walkthrough is the test; passing-with-known-bugs is more honest than walkthrough-with-on-the-fly-fixes.
- This is the integration gate for the whole epic. If it passes, the epic ships.

## Resolution

(Filled in when status → done.)
