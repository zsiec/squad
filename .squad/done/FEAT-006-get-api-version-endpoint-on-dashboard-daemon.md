---
id: FEAT-006
title: GET /api/version endpoint on dashboard daemon
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
captured_at: 1777290315
accepted_by: agent-401f
accepted_at: 1777290315
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by: []
---

## Problem

The MCP bootstrap pass (filed as FEAT-009) needs to ask the running dashboard daemon two things: what version is it, and where on disk does its binary live? Today, no endpoint exposes either. `squad serve --service-status` reads launchctl / systemctl, not the daemon's own state, so it cannot distinguish "daemon running" from "daemon running on a stale binary."

## Context

The dashboard server already lives in `internal/server/` with handlers wired through `server.New(...)` in `cmd/squad/serve.go`. The version string is in `cmd/squad/main.go` as `versionString`. `os.Executable()` returns the running binary path. All the data is at hand; we just need an endpoint.

Loopback gating is already a pattern in this server (see `prometheus.go` and the token gate in `serve.go`).

## Acceptance criteria

- [ ] `GET /api/version` returns 200 with JSON body `{"version": "...", "binary_path": "...", "started_at": "...", "pid": ...}`
- [ ] `version` reflects `versionString` baked into the binary
- [ ] `binary_path` reflects `os.Executable()` of the daemon process
- [ ] `started_at` is RFC3339, captured at server boot
- [ ] `pid` is `os.Getpid()`
- [ ] Endpoint is reachable on loopback only; non-loopback callers get 404 (matching the pattern used elsewhere in the dashboard)
- [ ] Unit test in `internal/server/` covers happy path + non-loopback rejection

## Notes

Keep the response shape stable — FEAT-009 will key its probe on it, and TASK-039 will assert on it. Do not include the restart token here; that lives behind FEAT-007's auth wall.

## Resolution
(Filled in when status → done.)
