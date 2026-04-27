---
id: FEAT-009
title: probe and ensure-daemon logic in bootstrap
type: feature
priority: P1
area: mcp
epic: first-run-dashboard
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290335
accepted_by: web
accepted_at: 1777290579
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by:
  - FEAT-006
  - FEAT-007
  - FEAT-008
---

## Problem

The bootstrap needs to detect daemon presence and version, install if absent, restart on version mismatch, and reinstall on binary-path drift. None of this logic exists today — FEAT-008 only created the seams. This item fills them in.

## Context

FEAT-006 provides the version probe target. FEAT-007 provides the restart endpoint. `internal/tui/daemon/` provides `Install` / `Uninstall` / `Reinstall` / `Status`. The remaining work is the orchestration: what call do you make in each combination of probe outcomes?

Concurrency: multiple Claude Code sessions can call `Ensure` simultaneously on a clean machine. A file lock on `~/.squad/install.lock` serialises the install attempts; loser sees the daemon up via the probe and treats it as a no-op.

## Acceptance criteria

- [ ] `Probe(ctx)` calls `GET http://127.0.0.1:7777/api/version` with a 500ms timeout and returns `ProbeResult{Present, Version, BinaryPath, StartedAt, PID}`
- [ ] `Ensure(ctx, opts)` orchestrates:
    - probe absent → file-lock `~/.squad/install.lock` → `opts.Manager.Install(...)` → poll `/api/version` until reachable (10s max, 200ms cadence)
    - probe present, `Version != opts.Version` → read `~/.squad/restart.token` → POST `/api/_internal/restart` with header → poll until version matches (10s max)
    - probe present, `BinaryPath != opts.BinaryPath` → `opts.Manager.Reinstall(...)` → poll until reachable
    - probe present, version + path match → no-op, returns nil
- [ ] `SQUAD_NO_AUTO_DAEMON=1` env var short-circuits `Ensure` to nil at the very top
- [ ] On `daemon.ErrUnsupported` or any install / restart error, `Ensure` logs to stderr and returns the wrapped error; never panics, never blocks indefinitely
- [ ] Unit tests cover all four probe outcomes using a fake `daemon.Manager` and an `httptest.Server` standing in for the daemon

## Notes

The 10s poll budget is enough for a cold launchd bootstrap on a busy laptop. Don't make it configurable — adding a knob now becomes documentation rot later. If it turns out to be too short, raise it as a code change with an evidence comment.

The token read happens lazily in the version-mismatch branch; do not pre-read it. If the file is missing on a path that requires it, log + return error (the daemon must have been installed by an old version that didn't write the token; a fresh `Reinstall` will create it).

## Resolution
(Filled in when status → done.)
