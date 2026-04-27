---
id: BUG-020
title: graceful skip of daemon install on Windows and unsupported platforms
type: bug
priority: P2
area: mcp
epic: first-run-dashboard
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290349
accepted_by: agent-401f
accepted_at: 1777290349
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to:
  - FEAT-009
blocked-by: []
---

## Problem

`internal/tui/daemon/install_other.go` returns `daemon.ErrUnsupported` on platforms outside macOS/Linux (notably Windows). Without a graceful path, the bootstrap on a Windows host would surface a noisy error in every Claude Code session — and potentially block tool serving if FEAT-009 doesn't handle the error correctly.

The user-facing behaviour we want: log once, continue, never panic. The user gets a banner that explains the situation and a hint to run `squad serve` manually.

## Context

`daemon.ErrUnsupported` is a sentinel; check via `errors.Is`. The bootstrap path needs to catch it specifically (rather than treating it as a transient error worth retrying) and emit the right banner copy.

This item is filed as `bug` because shipping FEAT-009 without it would create observable broken behaviour on a supported (if niche) platform.

## Acceptance criteria

- [ ] `bootstrap.Ensure` checks `errors.Is(err, daemon.ErrUnsupported)` after `Install`/`Reinstall` calls
- [ ] On match: log a one-line stderr hint (`squad: dashboard auto-install not supported on this platform; run "squad serve" manually for the UI`), set the banner to `Squad dashboard auto-install not supported on this platform; run "squad serve" manually`, return nil (not error)
- [ ] No retries on `ErrUnsupported`
- [ ] Bootstrap continues to skip Welcome on this path (no auto-open if the daemon isn't there)
- [ ] Unit test on a fake `daemon.Manager` that returns `ErrUnsupported` exercises this path: log emitted, banner set, no panic, `Ensure` returns nil

## Notes

When Windows support lands one day, this item's behaviour just stops triggering — no migration needed because it short-circuits gracefully on `ErrUnsupported` only.

## Resolution
(Filled in when status → done.)
