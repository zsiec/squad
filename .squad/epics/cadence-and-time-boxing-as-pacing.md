---
title: Cadence and time-boxing as pacing
spec: agent-team-management-surface
status: open
parallelism: parallel
dependencies: []
intake_session: intake-20260427-44256e4424c4
---

## Body

Humans pace themselves through habit. Agents do not — they keep working
until interrupted. The team relies on two pacing mechanisms today, both
documented but neither enforced:

- The 2h time-box on exploratory work, written into `CLAUDE.md` and the
  `squad-time-boxing` skill. Nothing wakes an agent at the cap.
- The chat verb taxonomy (`thinking` / `milestone` / `stuck` / `fyi` /
  `ask`), described by the `squad-chat-cadence` skill. A recent audit of
  chat traffic showed verb usage is bottom-heavy: `claim`, `done`,
  `release`, and `say` dominate; `thinking`, `milestone`, and `stuck`
  are absent from recent traffic. Peers cannot route attention if state
  is never posted.

Both rules are aspirational without an external prompt. This epic
converts them into mechanical async wakeups, and adds a per-agent
quality signal so operators can see who is finishing vs spinning.

The epic decomposes into three parallel items:

- **FEAT-042** schedules cadence wakeups at 90m (warning) and 120m
  (escalate-or-handoff) of a held claim, mirroring the documented 2h
  cap.
- **FEAT-043** adds a stale-chat nudge that fires every 30m on a held
  claim with no `thinking` or `milestone` since claim time, suppressible
  via `SQUAD_NO_CADENCE_NUDGES=1` for consistency with existing nudges.
- **FEAT-044** surfaces a done-to-release ratio per agent in
  `squad stats`, so operators can spot agents who release without
  closing.

### Success signal

In a fresh dogfood session, `milestone` and `thinking` posts appear at
the expected cadence without explicit user prompting, and a long-running
claim either escalates or hands off at the 2h cap on its own.
