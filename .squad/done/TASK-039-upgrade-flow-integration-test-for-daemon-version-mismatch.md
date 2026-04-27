---
id: TASK-039
title: upgrade-flow integration test for daemon version mismatch
type: task
priority: P2
area: mcp
epic: first-run-dashboard
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290349
accepted_by: web
accepted_at: 1777290588
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by:
  - FEAT-010
---

## Problem

The version-mismatch restart path is the most subtle piece of the bootstrap and the easiest to regress silently. If the probe stops returning the version, the restart endpoint stops being called, or the poll-until-version-match drifts, the user upgrades their binary and silently keeps running the old daemon. We need a test that owns this scenario end-to-end.

## Context

The mechanism is: `Probe` reports daemon version A → `Ensure` sees mismatch with the configured version B → reads `~/.squad/restart.token` → POSTs `/api/_internal/restart` → polls `/api/version` until it returns B → upgrade banner is set.

This test stages a fake "daemon at version A" via `httptest.Server`, then swaps it for "daemon at version B" when the restart endpoint is hit, and asserts the bootstrap rolls all the way through.

## Acceptance criteria

- [ ] Test in `internal/mcp/bootstrap/upgrade_test.go`
- [ ] Stage 1: `httptest.Server` returns `{version: "A", binary_path: "/old/squad", ...}` from `/api/version`; `~/.squad/restart.token` exists
- [ ] Stage 2: bootstrap invoked with `Options.Version = "B"`, `Options.BinaryPath = "/old/squad"`
- [ ] Asserts: probe sees mismatch → POST `/api/_internal/restart` invoked with the correct token in the header → server's restart handler swaps the version field to "B"
- [ ] Asserts: poll succeeds at `/api/version` returning "B" → bootstrap returns nil → `ConsumeBanner` returns the upgrade copy
- [ ] **Path-drift case**: same test setup but with `Options.BinaryPath = "/new/squad"` → asserts `Manager.Reinstall` is invoked (not just restart) before the version poll
- [ ] **Poll timeout case**: mock the server to never flip its version → bootstrap returns error after 10s; test uses a smaller budget injected via test seam

## Notes

Don't shell out to actual launchd / systemd in this test. The fake `daemon.Manager` and the `httptest.Server` cover the contract.

## Resolution
(Filled in when status → done.)
