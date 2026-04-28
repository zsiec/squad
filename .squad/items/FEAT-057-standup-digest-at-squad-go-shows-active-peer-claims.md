---
id: FEAT-057
title: standup digest at squad go shows active peer claims
type: feature
priority: P2
area: cmd
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777345158
accepted_by: web
accepted_at: 1777345477
references: []
relates-to: []
blocked-by: []
epic: team-practices-as-mechanical-visibility
---

## Problem

`squad go` registers the agent, claims the top ready item, and prints
the AC, but says nothing about peer activity. The agent enters the
work loop with no awareness of who else is holding what, in which
area, or how recently. Result over the last 2 days: 126 of 182 claims
(69%) had zero interim human posts, in part because the silent path
costs nothing and the framework offers no friction at the moment a
peer collision could be cheaply caught.

## Context

The data is already in the global SQLite ledger. `internal/claims/`
exposes active claims; `internal/chat/` carries the most recent
human-typed post per thread (kinds: thinking / milestone / fyi / ask
/ stuck / say / answer). `cmd/squad/go.go` is the one entrypoint
every session passes through, so the digest only needs to land
there — no new daemon path required.

Worktree-per-claim made the visibility gap worse, not better:
collisions that used to surface as immediate merge pain now hide
until fold-time, so an agent claiming `area: spa` while a peer is
mid-flight on the same area gets no early signal.

## Acceptance criteria

- [ ] `squad go` prints a `peers:` block before the claim's AC line.
      Each row: agent display name, item id, age (last_touch
      relative to now), area, and the most recent human-verb post
      excerpt (truncated to ~80 chars). If no peers are active, the
      block prints `peers: none active.` and one trailing blank
      line.
- [ ] When the new claim's `area:` matches any active peer's `area:`,
      the digest precedes the AC with a one-line nudge:
      `overlaps with @<peer> on <ID> (area=<X>) — ack with 'squad ask
      @<peer> ...' before starting`. Just informational; not a hard
      gate.
- [ ] Sort order is most-recently-touched first. Cap the rendered
      list at 6 rows with a `… (+N more)` line if there are more.
- [ ] A unit test asserts the rendered output for: zero peers, one
      peer no overlap, one peer with area overlap (nudge present),
      seven peers (truncated with `+1 more`).
- [ ] Single-repo dogfood smoke: running `squad go` against a DB
      with two seeded peer claims produces the expected block —
      paste the output line in the resolution.

## Notes

- Source the "human verb" filter from the existing chat-kind set
  used by `squad-chat-cadence` so this stays in sync if new verbs
  are added later.
- This item is the prerequisite for the rest of the epic. Standup
  surfaces the data the retro will later synthesize.

## Resolution
(Filled in when status → done.)
