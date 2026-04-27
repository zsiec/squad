---
id: FEAT-007
title: POST /api/_internal/restart endpoint on dashboard daemon
type: feature
priority: P1
area: server
epic: first-run-dashboard
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290335
accepted_by: agent-401f
accepted_at: 1777290335
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by: []
---

## Problem

When the MCP bootstrap detects a binary version mismatch with the running daemon, it needs a way to ask the daemon to exit cleanly so launchd / systemd-user can relaunch it on the new binary. There is no such endpoint today; the only way to restart the daemon is `squad serve --reinstall-service`, which tears down and re-creates the plist when all we want is a process bounce.

## Context

`internal/tui/daemon/` already does the heavy lifting for service install / uninstall / reinstall. launchd's `KeepAlive` and systemd-user's `Restart=always` will relaunch the binary as soon as it exits. So the daemon-side mechanism is just: receive a request, schedule `os.Exit(0)`, let the OS service supervisor do its job.

The auth concern is real. Any process on the loopback could otherwise hit the restart endpoint repeatedly and DoS the daemon. A per-machine token written by `Install` and consumed by the bootstrap fixes this without introducing a new key-management surface.

## Acceptance criteria

- [ ] `POST /api/_internal/restart` requires header `X-Squad-Restart-Token: <token>` matching the contents of `~/.squad/restart.token`
- [ ] Token file is created during `daemon.New().Install(opts)` if missing, mode 0600, contents are 32 bytes of `crypto/rand` hex
- [ ] On valid token, daemon returns 202, flushes the response, then schedules `os.Exit(0)` after 200ms (`time.AfterFunc`)
- [ ] On missing or wrong token: 401 with `{"error": "invalid token"}`
- [ ] Endpoint is reachable on loopback only; non-loopback callers get 404
- [ ] Unit test exercises: valid token → exit-scheduler invoked; invalid token → 401, no exit; loopback gate rejects

## Notes

The 200ms delay matters — without it, the response can race the process exit and the caller sees a connection reset instead of a 202. Use a fake exit function in tests so the test process doesn't actually exit.

## Resolution
(Filled in when status → done.)
