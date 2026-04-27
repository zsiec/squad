---
id: CHORE-003
title: tool-arg redaction config + privacy doc
type: chore
priority: P2
area: cli
status: open
estimate: 45m
risk: medium
evidence_required: []
created: 2026-04-27
updated: 2026-04-27
captured_by: agent-bbf6
captured_at: 1777251711
accepted_by: agent-bbf6
accepted_at: 1777251711
epic: agent-activity-stream
references:
  - cmd/squad/event.go
  - internal/config/config.go
  - docs/reference/
relates-to:
  - TASK-022
blocked-by:
  - TASK-022
---

## Problem

`squad event record` writes raw `--target` payloads. Tool args can contain file paths, secrets, environment values, internal hostnames — leaking these into the durable `agent_events` table is a security/privacy concern. This chore adds a redaction layer between hook and DB, plus a docs page warning operators about the privacy posture of the events table.

## Context

`cmd/squad/event.go` (TASK-022) accepts `--target` and inserts as-is. We need a `redact()` helper that:
- Truncates to 200 chars by default
- Drops payloads matching a configured regex (e.g. `^.*(password|secret|token)=.*$`)
- Honors `SQUAD_REDACT_REGEX` env var as the override
- Falls back to a per-repo `.squad/config.yaml` `events.redact_regex` field
- Logs the redaction count to stderr (so operators can spot if it's firing too aggressively)

The privacy doc lands at `docs/reference/agent-events-privacy.md` (or extends an existing reference page). Must say plainly: events are operator telemetry, do not expose over public webhooks, the table contains file paths and command shapes that may reveal repo structure.

## Acceptance criteria

- [ ] New `redact(s string, cfg redactConfig) (out string, redacted bool)` function in `cmd/squad/event.go` (or a new `cmd/squad/event_redact.go`):
  - Truncates to `cfg.MaxLen` (default 200, 0 = unlimited)
  - If `cfg.Pattern` matches, returns the literal string `"<redacted>"` and `redacted=true`
  - Returns the input unchanged otherwise
- [ ] `redactConfig` resolved from (highest priority first): `SQUAD_REDACT_REGEX` env var → `.squad/config.yaml` `events.redact_regex` field → empty (no pattern, just truncation).
- [ ] `cmd/squad/event.go` `record` subcommand calls `redact()` on `--target` before insert.
- [ ] New `Events` block on `config.Config` struct (yaml tag `events`) with `RedactRegex` (yaml tag `redact_regex`) and `MaxTargetLen` (yaml tag `max_target_len`).
- [ ] Unit tests in `cmd/squad/event_redact_test.go`: truncation only, regex match, regex no-match, env override beats config, config falls back to default.
- [ ] New doc at `docs/reference/agent-events-privacy.md`:
  - One-paragraph overview of what's in `agent_events`
  - One paragraph on the redaction config knobs
  - Explicit "do not expose this table publicly" warning
  - Pointer to the SPA's authentication posture
- [ ] `go test ./cmd/squad/...` passes; trailing `ok` line pasted.

## Notes

- `evidence_required: []` because this is a chore that DOES touch Go code — but the test coverage is already in scope (`event_redact_test.go`). The Go code path is tested; the docs file is verified by reading.
- Defaults are deliberately conservative (truncate to 200, no regex). Operators who want stricter redaction set their own regex.
- If the redaction fires on every event, that's a sign the regex is too broad — log to stderr (not chat, hooks must stay quiet) so the operator can spot it without flooding chat.
- Don't redact `--tool`, `--kind`, `--exit`, etc. — only `--target` carries arg payloads.

## Resolution

(Filled in when status → done.)
