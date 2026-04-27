---
id: FEAT-008
title: bootstrap package skeleton in internal/mcp/bootstrap
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

Three downstream items (FEAT-009 probe+ensure, FEAT-011 welcome, FEAT-012 banner) all want to land in parallel against a shared `internal/mcp/bootstrap/` package. There is no such package today, and without seams in place the parallel work would either step on itself or serialise behind one author. This item creates the skeleton — files, types, function signatures — so the three follow-on items have stable handles to fill in.

## Context

Spec mcp-dashboard-bootstrap and design doc 2026-04-27 specify the package layout. Test isolation requires the daemon manager to be injectable (real `daemon.Manager` in production, fake in tests).

## Acceptance criteria

- [ ] New directory `internal/mcp/bootstrap/` with files `probe.go`, `ensure.go`, `welcome.go`, `banner.go`
- [ ] `Ensure(ctx context.Context, opts Options) error` exists in `ensure.go` with stub body returning nil
- [ ] `Probe(ctx context.Context) (ProbeResult, error)` exists in `probe.go` with stub returning a zero ProbeResult
- [ ] `Welcome(ctx context.Context) error` exists in `welcome.go` with stub returning nil
- [ ] `SetBanner(s string)` and `ConsumeBanner() string` exist in `banner.go` and use `atomic.Value` for consume-and-clear semantics
- [ ] `Options` struct holds at minimum `BinaryPath`, `Bind`, `Port`, `HomeDir`, `Manager daemon.Manager` (so tests inject a fake)
- [ ] Smoke test in `bootstrap_test.go` verifies the package compiles and each function is callable
- [ ] No new behaviour beyond the seams — that lands in FEAT-009/011/012

## Notes

Keep dependencies thin: import `internal/tui/daemon`, `context`, `sync/atomic`. Do not import `net/http` here — the probe lives in `probe.go` and is filled in by FEAT-009. Do not import `os/exec` — that lands in FEAT-011's welcome.go.

## Resolution
(Filled in when status → done.)
