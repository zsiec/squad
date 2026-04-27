---
spec: agent-activity-stream
status: open
parallelism: |
  Foundation: schema (#1) + CLI recorder (#2) must land first since every hook and
  endpoint downstream consumes them. After that:
  - Hook wire-ups (#3, #4) can land in parallel with each other and with the API work (#5)
    once the recorder exists — they touch different files (plugin/hooks/*.sh vs internal/server/).
  - SPA work (#6, #7, #8) serializes in itself (drawer scaffolding → timeline render → SSE)
    but is independent of the hook wire-ups.
  - Redaction config (#9) is foundation-adjacent; lands once #2 exists.
  - Dogfood (#10) is the integration gate; blocks on everything.

  Recommended dispatch: #1 first (blocks all). Then #2 (blocks recorder consumers).
  Then #3, #4, #5, #9 in parallel. Then #6 → #7 → #8 in series. Then #10 last.
  Three-agent chain works well; four would let one agent run the full SPA series
  while two split the hook + API + redaction work.
---

## Goal

Turn the latent signal already firing through squad's hook system into a per-agent activity stream the SPA can render as a click-through drawer, with live SSE updates, configurable filtering, and arg redaction. After this epic ships, an operator clicking on an agent in the SPA sees what that agent is doing this second, not just its registration metadata.

## Child items

10 implementation tasks. Each item's acceptance criteria are written so a fresh agent can execute without reading the spec.

## Anti-patterns to avoid during execution

- **No tool-output capture.** Only metadata (timestamp, kind, tool, target, exit, duration). Output is too large and too sensitive.
- **No retroactive backfill.** Events start when the hooks are wired; prior session history lives in chat/claims/commits and can be read from the existing rollups.
- **No new authentication surface.** The events endpoints inherit whatever auth the existing `/api/*` endpoints use. If those are unauthenticated today, that's a separate item, not this epic's problem.
- **No PM traces in source artifacts.** Same convention as the rest of the squad codebase.
- **No per-event attestation.** Attestations remain explicit/gated. Events are unverified telemetry.

## Phasing inside the epic

The schema (#1) plus recorder (#2) is "Phase A" — once those land, hook wire-ups and the read API can advance in parallel. SPA work (Phase B) reads the API and is independent of which hooks have wired up. Phase C is the live SSE channel. Phase D is the dogfood walkthrough. The phasing is implicit in the `blocked-by` chains; no need to call it out further.
