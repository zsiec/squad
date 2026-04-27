---
id: FEAT-013
title: symmetric daemon uninstall in install-plugin --uninstall
type: feature
priority: P1
area: plugin
epic: first-run-dashboard
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290336
accepted_by: agent-401f
accepted_at: 1777290336
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by: []
---

## Problem

`squad install-plugin --uninstall` removes plugin assets, the `mcpServers` entry, and the registered hooks — but leaves a launchd / systemd-user dashboard service running. After auto-install ships in this epic, that becomes user-visible orphan state: the user thinks they uninstalled squad, but a background daemon keeps running and a plist sits in `~/Library/LaunchAgents`.

## Context

`cmd/squad/install_plugin.go` already has the uninstall branch. `internal/tui/daemon/` already has `Uninstall()`. Wiring the call in is one short addition. The two squad-managed sentinels (`~/.squad/.welcomed`, `~/.squad/restart.token`) should also be removed for full symmetry.

User data is **not** removed: `~/.squad/global.db` and any repo `.squad/` directories survive. They contain claims, items, attestations, and learnings that may have value beyond the plugin lifecycle.

## Acceptance criteria

- [ ] `cmd/squad/install_plugin.go` `--uninstall` path calls `daemon.New().Uninstall()` after the existing teardown
- [ ] Uninstall failure logs a stderr warning but does not abort the rest of the cleanup
- [ ] `~/.squad/.welcomed` is removed if present (`os.Remove`, ignore `os.ErrNotExist`)
- [ ] `~/.squad/restart.token` is removed if present
- [ ] `~/.squad/global.db` is **not** touched
- [ ] Repo-local `.squad/` directories are **not** touched
- [ ] Integration test asserts a full install → uninstall round-trip leaves no plist, no unit, no welcome sentinel, no restart token; assert global.db is preserved

## Notes

Symmetry with the install path matters because users will mistake leftover daemon state for "squad uninstall is broken." Make the install vs uninstall calls live next to each other in the source so future maintainers see them as a pair.

## Resolution
(Filled in when status → done.)
