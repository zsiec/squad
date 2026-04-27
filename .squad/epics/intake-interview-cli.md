---
spec: intake-interview
status: open
parallelism: |
  Single-task epic. Depends on intake-interview-core. Can land in parallel
  with intake-interview-mcp once both have a shared internal/intake API.
---

## Goal

Add a `squad intake` cobra subcommand tree that exposes the lifecycle to
humans and scripts: kick off interviews, list/inspect/cancel sessions, and
provide an emergency manual commit form.

## Scope

- `squad intake new <idea...>` — kick off green-field session.
- `squad intake refine <item-id>` — kick off in refine mode.
- `squad intake list` — open sessions for current (repo, agent).
- `squad intake status <id>` — pretty-print transcript + checklist gaps.
- `squad intake cancel <id>` — mark cancelled.
- `squad intake commit <id> --bundle <path>` — last-resort scriptable commit.
- Wire into the root cobra command.
- Integration tests that drive the full lifecycle end-to-end.

## Out of scope

- The interview prose itself — that lives in the plugin skill, not the CLI.
- Stateful CLI session resumption beyond what `intake.Status` already provides.
