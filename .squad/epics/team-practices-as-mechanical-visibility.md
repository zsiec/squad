---
title: Team practices as mechanical visibility
spec: agent-team-management-surface
status: open
parallelism: |
  Filled in by 'squad analyze team-practices-as-mechanical-visibility' — do not hand-edit.
---

## Goal

Adapt high-leverage human software-team practices — standup, code
ownership, risk-tiered review, retrospective, postmortem — into the
agent realm by replacing the social pressure that drives them in
human teams with mechanical visibility and ledger-state gates that
agents respond to.

## Observation

Audit of the chat ledger over the most recent two days shows the gap
is in the *middle* of work, not the metadata around it:

- 1,104 auto-broadcast verbs (claim/release/done) vs 200 human verbs
  (thinking/milestone/fyi/ask/stuck/say/answer) — 5.5:1.
- 126 of 182 claims (69%) have zero interim human posts. Agent
  claims, works silently, broadcasts done.
- `ask` was used 7 times. `stuck` 5 times. `knock` once. Active peer
  coordination is essentially nonexistent.
- Mentions appear in 70 of 1,304 messages (5%).

The pattern is consistent with a structural cause, not a discipline
gap: the framework prescribes cadence, doesn't measure it, and gives
the silent path the lowest friction. Worktrees compounded this —
conflicts that used to surface as immediate merge pain now hide
until fold-time, so posting "touching X" feels redundant in the
moment.

The asymmetries from the parent spec apply: agents do not respond to
"peers will notice you're silent" because they don't model peers
socially; agents do respond to ledger artifacts and explicit gates.
Practices that worked in human teams via culture must be re-cast as
practices that work in the agent realm via plumbing.

## Items in this epic

- **FEAT — standup digest at `squad go`.** When `squad go` runs,
  before announcing the new claim's AC, print a one-line peer
  digest: each active claim's agent, item, age, area, and last
  human-posted excerpt (truncated). If the new claim's area
  overlaps an active peer's, prompt to ack/ask before starting.
  Replaces "morning standup" with passive ambient awareness at the
  one moment every agent already runs through.

- **FEAT — risk-tiered code review.** Items with `priority: P0` or
  `risk: high` require *two* independent review subagents before
  `squad done` will close: the existing code-quality reviewer and a
  second adversarial pass framed as "what fails in production." Both
  must greenlight. Lower-tier items keep today's single review.
  Replaces "ensemble programming on critical changes" with mechanical
  gating only where stakes warrant.

- **FEAT — retro generator (`squad retro`).** Weekly cron-driven (or
  manually invoked) command that reads the last 7 days of items,
  chats, blockers, and attestations and writes
  `.squad/retros/YYYY-WW.md` with: top 3 failure modes, slowest item
  type to close, and one process recommendation. Operator reads;
  agents auto-load via skill paths glob. Replaces "blameless retro
  meeting" — the data is in the SQLite ledger; the missing piece is
  durable synthesis.

- **FEAT — area-ownership auto-mention.** When a new item is filed
  with an `area:` set, auto-`fyi` the agent who closed the most
  items in that area in the last 30 days. When a claim crosses an
  area boundary or scope-splits, auto-`fyi` the same way. Replaces
  GitHub CODEOWNERS — gives the system a "who's the expert here"
  signal even when the operator can't manually route.

- **FEAT — auto-postmortem on long blocks.** When an item closes as
  `blocked` or `released` after >2h of held-claim time, dispatch a
  follow-up agent under a `postmortem` flag that writes a structured
  artifact to `.squad/learnings/<slug>.md`: hypotheses tried,
  ruled-out causes, evidence collected, what to do differently.
  Replaces "blameless postmortem after incident" — captures the
  lesson before the chat thread is cleaned up.

## Sequencing

Standup digest first — biggest behavior change for the smallest code
change, and surfacing peer state at the one moment every agent passes
through is the prerequisite for the others to land in context.

Retro generator second — once the standup makes peer state passive,
the retro produces the durable record that lets the operator see
whether silence-detection alone moves the chat ratio. The
recommendations the retro emits will inform whether the remaining
items need to ship at all, or in what order.

Risk-tiered review, area auto-mention, and auto-postmortem can land
in parallel after retro is producing usable output, since each
exercises a different code path and a different default.

## Non-goals

- Adding more chat verbs to the cadence menu. The existing verbs are
  not the bottleneck; the structures that consume them are.
- Simulating human-team social dynamics: ego, mentorship,
  watercooler rapport. Agents do not respond to these and the
  attempt to bolt them on adds friction without uptake.
- Replacing the operator's judgment on item priority or scope. These
  practices surface signal; the operator still decides.
