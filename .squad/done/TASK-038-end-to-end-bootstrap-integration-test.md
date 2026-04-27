---
id: TASK-038
title: end-to-end bootstrap integration test
type: task
priority: P2
area: mcp
epic: first-run-dashboard
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290349
accepted_by: web
accepted_at: 1777290586
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

The first-run flow has a lot of moving parts: probe, install, poll, welcome, sentinel, banner. Each is unit-tested, but a black-box test that exercises the whole bootstrap sequence is the only way to catch regressions where one piece silently stops calling another.

## Context

The unit tests for FEAT-009/011/012 cover their respective surfaces. This integration test covers the **interaction**: a clean home dir, no daemon, what does the full `Ensure → Welcome → first tools/call` sequence produce?

Test fixtures: a fake `daemon.Manager` that records `Install` calls; an `httptest.Server` that comes up *after* `Install` is invoked and serves `/api/version`; a tempdir for `~/.squad/`; an injectable opener that records the URL it was asked to open.

## Acceptance criteria

- [ ] Test in `internal/mcp/bootstrap/integration_test.go` (or `cmd/squad/mcp_e2e_test.go`)
- [ ] **Clean-machine path**: probe says absent → fake `Install` invoked → poll succeeds → welcome fired → sentinel written → banner set → first `tools/call` response carries the banner
- [ ] **Re-run path**: same test setup but with sentinel already present and daemon already up at the right version → no install, no welcome, no banner
- [ ] **Opt-out paths**: `SQUAD_NO_AUTO_DAEMON=1` short-circuits cleanly (no install attempted); `SQUAD_NO_BROWSER=1` writes sentinel but does not call the opener
- [ ] **Failure paths**: `Install` returns error → bootstrap logs and returns error; MCP serving still proceeds (test asserts the JSON-RPC server still answers `initialize`)

## Notes

Don't test version-mismatch here — that's TASK-039's job. This test is for the absent-daemon flow.

Keep the test isolated: `t.TempDir()` for HOME, no shared global state between subtests.

## Resolution
(Filled in when status → done.)
