---
id: FEAT-011
title: welcome sentinel and auto-open browser on first run
type: feature
priority: P1
area: mcp
epic: first-run-dashboard
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290336
accepted_by: web
accepted_at: 1777290583
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by:
  - FEAT-008
---

## Problem

After the daemon is installed by `Ensure`, the user still has to know the URL and open a browser. We need a one-shot auto-open flow gated on a sentinel so it never re-opens — the user sees the page on first run and never again.

## Context

Cross-platform open: `open <url>` on macOS, `xdg-open <url>` on Linux. Windows is out of scope (BUG-020 handles that gracefully). The sentinel `~/.squad/.welcomed` is a plain zero-byte file checked with `os.Stat`.

Operators in CI / headless environments need an opt-out — `SQUAD_NO_BROWSER=1`. They still want the sentinel written so subsequent runs don't try to open either.

## Acceptance criteria

- [ ] `bootstrap.Welcome(ctx)` checks `~/.squad/.welcomed`:
    - present → return nil immediately
    - absent → invoke platform-appropriate open command (`open` darwin, `xdg-open` linux), then write the sentinel (zero-byte, mode 0644)
- [ ] `SQUAD_NO_BROWSER=1` skips the open invocation but still writes the sentinel
- [ ] Sentinel write failure logs a stderr warning but does not return an error (don't block bootstrap on a transient FS issue)
- [ ] Open command failure logs a stderr warning but does not block; sentinel is still written so we don't retry
- [ ] Unit test asserts: sentinel absent → open invoked once → sentinel written; sentinel present → no-op; `SQUAD_NO_BROWSER=1` → no open, sentinel written
- [ ] `os/exec` is mocked via an injectable opener function so tests don't actually open a browser

## Notes

URL is hardcoded `http://localhost:7777` for now. If the daemon binds to a non-default port one day, plumb that through `Options` then.

## Resolution
(Filled in when status → done.)
