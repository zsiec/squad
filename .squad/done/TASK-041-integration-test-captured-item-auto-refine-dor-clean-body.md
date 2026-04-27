---
id: TASK-041
title: 'integration test: captured item -> auto-refine -> DoR-clean body'
type: task
priority: P2
area: server
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777308128
accepted_by: web
accepted_at: 1777308351
epic: auto-refine-inbox
references:
  - internal/server/integration_refine_test.go
relates-to: []
blocked-by: [FEAT-029, FEAT-030]
---

## Problem

The auto-refine flow involves an items primitive, an MCP tool, and an HTTP endpoint that wraps a subprocess. We need one end-to-end integration test that exercises the full path — captured item with placeholder AC → POST /api/items/{id}/auto-refine → rewritten item file → DoRCheck returns zero violations — without depending on a real `claude` subprocess (CI cannot run the CLI).

## Context

The pattern to follow is `internal/server/integration_refine_test.go`, which exercises the manual-refine loop end-to-end. For auto-refine, the subprocess is the obstacle: we cannot launch a real CLI in CI. The test injects a fake `commandRunner` (the package-level seam from FEAT-030) that, instead of running `claude`, calls `squad_auto_refine_apply` directly via the in-process MCP server with a canned drafted body. That exercises every server-side seam (handler, dedupe, narrow MCP config build, post-call verification, response shape) except the actual CLI fork — which is also the only piece we cannot test in CI.

## Acceptance criteria

- [ ] New `internal/server/integration_auto_refine_test.go` with one or more sub-tests under a top-level test function.
- [ ] Setup: bootstrap a server with one captured item whose body is the squad-new template defaults (title-only, placeholder AC); confirm pre-state by asserting `items.DoRCheck` returns the `template-not-placeholder` violation.
- [ ] Inject a fake `commandRunner` (or a fake exec factory — whatever seam FEAT-030 exposed) that, when invoked, calls `squad_auto_refine_apply(item_id, <DoR-clean canned body>)` synchronously and exits 0; asserts on the way through that the prompt and MCP config path were passed correctly.
- [ ] POST `/api/items/{id}/auto-refine`, assert 200, assert response JSON includes the rewritten body and `auto_refined_at > 0`.
- [ ] Re-parse the on-disk file; assert `auto_refined_at` advanced, `auto_refined_by == "claude"`, status still `captured`, and `items.DoRCheck` now returns zero violations.
- [ ] Sub-test for "claude exited but did not call the tool" — fake commandRunner exits 0 without calling auto_refine_apply; assert handler returns 500 and the file is untouched.
- [ ] Sub-test for the dedupe path — start one in-flight click (fake commandRunner blocks until released), POST a second click while the first is pending, assert 409; release the first, assert it completes 200.
- [ ] Sub-test for non-captured status — flip an item to `open` first, POST auto-refine, assert 409 with the status-mismatch error message.
- [ ] All sub-tests run with `go test -race` clean.

## Notes

If FEAT-030 chose to use `os.Setenv` or a similar global to swap the runner, the test must clean up via `t.Cleanup` to avoid bleeding into other tests. The fake commandRunner also needs to mimic the prompt/config path arguments closely enough that any future test asserting on those passes — recommend a small struct that records every invocation for assertion.
