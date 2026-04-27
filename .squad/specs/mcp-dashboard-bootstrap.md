---
title: MCP-driven dashboard bootstrap and upgrade hygiene
motivation: |
  Today the user installs the binary, installs the plugin, then has to open a separate
  terminal and run `squad serve` before they ever see the dashboard. The savvy-novice
  user does not know `squad serve` exists and does not know to manage its lifetime by
  hand. Layered on top, when the binary is upgraded the dashboard daemon keeps running
  the old version until someone notices. Both problems can be solved without any new
  user-facing concepts: the MCP server already boots in every Claude Code session, and
  squad already ships a launchd / systemd-user service installer. Wiring the two
  together gives a zero-step first-run UX and a transparent upgrade path.
acceptance:
  - "A clean machine with the plugin newly installed: the user opens Claude Code, types anything that triggers an MCP tool call, the dashboard opens automatically in the default browser, and the URL appears as a banner in Claude's reply — without any explicit `squad serve` step."
  - "Re-opening Claude Code in any subsequent session does not re-open the browser, does not re-install the service, and does not re-emit the banner. The first-run experience happens once per machine."
  - "After `go install github.com/zsiec/squad/cmd/squad@latest`, the next MCP boot detects version mismatch with the running daemon, calls a token-gated restart endpoint, the daemon exits cleanly, launchd / systemd-user relaunches it on the new binary, and a one-line banner notes the upgrade."
  - "If the binary path moves (`os.Executable() != daemon.binary_path`), the next MCP boot reinstalls the service so the plist / unit points at the new path, then performs the version-restart flow."
  - "`squad install-plugin --uninstall` leaves no daemon, no plist / unit, no `~/.squad/.welcomed` sentinel, and no `~/.squad/restart.token`. Repo `.squad/` directories and `~/.squad/global.db` are untouched."
  - "`SQUAD_NO_AUTO_DAEMON=1` skips the install / restart flow; `SQUAD_NO_BROWSER=1` skips the auto-open but still writes the welcome sentinel."
  - "Bootstrap failures (port 7777 in use, plist permission denied, unsupported platform) log a stderr warning and let MCP continue serving tools — squad without a UI is still functional."
non_goals:
  - "Browser-side cache busting. Out of scope; can be filed separately if it surfaces in practice."
  - "Windows daemon support. Filed as a graceful-skip bug-class item rather than a fallback implementation."
  - "Schema migrations or DB cleanup as part of bootstrap. `store.OpenDefault()` keeps that responsibility."
  - "Auto-launching the OS service across reboots from MCP boot. The OS handles that once installed."
  - "Higher-level daemon health checks beyond the version probe. `squad doctor` already covers that surface."
integration:
  - "internal/mcp/bootstrap/ — new package: probe.go, ensure.go, welcome.go, banner.go"
  - "internal/server/ — two new endpoints: GET /api/version, POST /api/_internal/restart"
  - "internal/mcp/server.go — banner consume-and-clear hook into first tools/call response"
  - "cmd/squad/mcp.go — call bootstrap.Ensure() before starting the JSON-RPC loop"
  - "cmd/squad/install_plugin.go — symmetric daemon teardown on --uninstall"
  - "internal/tui/daemon/ — existing infra, called from the new bootstrap path"
  - "README.md, docs/adopting.md — replace step 4 with the zero-step flow"
---

## Background

The current Quick Start in the README has four steps: install the binary, install the plugin, open Claude Code in a project, then "tell Claude to run `squad serve`" or run it yourself. The fourth step is invisible to a brand-new user — the dashboard isn't mentioned in the welcome path of Claude Code, and `squad serve` isn't surfaced anywhere except the docs.

Squad already ships everything we need to fix this. `internal/tui/daemon/` has working launchd (macOS) and systemd-user (Linux) installers behind a `Manager` interface. `cmd/squad/serve.go` exposes `--install-service`, `--uninstall-service`, `--reinstall-service`, and `--service-status`. The dashboard runs on `127.0.0.1:7777` by default and serves the SPA the user already needs. What's missing is the trigger: nothing currently calls `daemon.New().Install()` automatically.

This spec wires the trigger into the MCP server boot path. The MCP server is the right place because it already runs in every Claude Code session, has access to the squad binary it was launched from, and has a structured way to surface output (the first tool response). The flow is:

1. **First MCP boot ever** → no daemon → install service → daemon starts → auto-open browser → banner in first tool response.
2. **Subsequent boots** → probe daemon → version matches → no-op.
3. **Boot after binary upgrade** → probe daemon → version mismatch → token-gated restart → poll for new version → upgrade banner.
4. **Boot after binary path change** → probe daemon → path mismatch → reinstall service → restart → upgrade banner.

The full design (architecture, edge cases, implementation sequence, success criteria) is at `docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md`.

## Reading the success bar

A successful rollout means the README "Quick start" can drop step 4 entirely — the dashboard is part of "install the plugin," not a separate concern. Concretely, after a clean install on a developer's machine:

| Action | Before | After |
|---|---|---|
| Steps to working dashboard | 3 (binary + plugin + manual `squad serve`) | 2 (binary + plugin) |
| Steps after binary upgrade | 1 (manual restart of the daemon) | 0 (transparent in next MCP boot) |
| User awareness of `squad serve` required | yes | no (still available for power users) |
