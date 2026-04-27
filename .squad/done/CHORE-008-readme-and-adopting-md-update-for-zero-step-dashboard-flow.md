---
id: CHORE-008
title: README and adopting.md update for zero-step dashboard flow
type: chore
priority: P2
area: docs
epic: first-run-dashboard
status: done
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290349
accepted_by: web
accepted_at: 1777290605
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by:
  - FEAT-010
  - FEAT-011
  - FEAT-012
---

## Problem

Once the auto-install flow ships, the docs that describe a manual `squad serve` step are misleading. New users will read the README, see "run `squad serve` in another terminal," and either do that pointlessly or wonder why they need to.

## Context

`README.md` "Quick start" lists three steps explicitly and then "ask Claude to start `squad serve`" in the "Live dashboard" paragraph below. `docs/adopting.md` walks through the same flow at length. `docs/recipes/prometheus.md` and `docs/troubleshooting.md` may also reference manual `squad serve` invocations — they need a review pass.

The `squad serve` command itself stays in the codebase and the docs — power users who want manual control, or who want to bind to a non-loopback address with a token, still need it. We're just retiring it from the **default** onboarding path.

## Acceptance criteria

- [ ] README "Quick start" reduces to two steps: install binary, install plugin
- [ ] "Live dashboard" section in README: replace manual-`squad serve` language with a description of the auto-install flow; document `SQUAD_NO_AUTO_DAEMON=1` and `SQUAD_NO_BROWSER=1` escape hatches
- [ ] `docs/adopting.md` updated to match the new flow
- [ ] `docs/recipes/prometheus.md` and `docs/troubleshooting.md` reviewed for stale references; either updated or left alone with explicit context (e.g., "if you've opted out of auto-install ...")
- [ ] No broken links anywhere in the modified files
- [ ] `squad serve --help` output unchanged (no code changes here, just docs)

## Notes

Lead with the new behaviour, mention the env-var opt-outs as power-user knobs at the end. Keep the README short — most power-user content lives in `docs/`.

## Resolution
(Filled in when status → done.)
