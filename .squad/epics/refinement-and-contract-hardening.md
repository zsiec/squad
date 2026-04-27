---
title: Refinement and contract hardening
spec: agent-team-management-surface
status: open
parallelism: serial
dependencies: []
intake_session: intake-20260427-44256e4424c4
---

## Notes

The intake interview was added to lift acceptance-criteria quality across the
team, but two structural gaps are blunting that goal:

1. The interview's captured content is silently dropped at commit time. For
   item-only and refine flows, `internal/intake/commit_run.go` only forwards
   `Ready` and `CapturedBy` into `items.NewWithOptions`, so the bundle's
   `Intent`, `Acceptance`, `Area`, and `Notes` never reach the resulting
   markdown. The agent answers the interview, then opens the file and finds
   the same `Specific, testable thing 1` template a non-interviewed item gets.
2. `squad accept`'s Definition of Ready (BUG-027) only catches the literal
   template placeholder strings. A bullet like "works correctly" passes today
   but is no more refutable than the placeholder it replaced. Refinement is
   the escape valve, but logs show it's reached for ~1% of items — agents
   route around it because the bar to clear `accept` is too low.
3. Items with many distinct sub-targets get claimed as one PR-sized unit and
   then either balloon or get partially completed. `squad decompose` exists
   but isn't surfaced at the moment of decision (the claim).

The three items below address each gap:

- `BUG-028` — make the intake commit actually render bundle bodies into the
  item markdown for both item-only and refine modes.
- `FEAT-036` — extend the DoR heuristic so vague AC bullets (too short, no
  verb, restating the title) are rejected at `squad accept`, gated to
  `feat`/`bug` types.
- `FEAT-037` — at `squad claim` time, nudge the agent to run
  `squad decompose <ID>` when the item has 4+ acceptance bullets that
  reference 3+ distinct files.

Why `serial`: `BUG-028` is foundational. Both `FEAT-036` and `FEAT-037`
inspect the item body — the AC bullets and the file references inside them.
Until the intake actually preserves what the user typed, `FEAT-036` would
fire on the template placeholders for every interviewed item (false
positive), and `FEAT-037` would never fire because no real file references
ever land in the body (false negative). Land `BUG-028` first; the other two
become meaningful only after the body is real.
