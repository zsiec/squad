---
id: CHORE-003
title: tool-arg redaction config + privacy doc
type: chore
priority: P2
area: cli
status: done
estimate: 45m
risk: medium
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
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

- [x] New `redact(s string, cfg redactConfig) (out string, redacted bool)`
  in `cmd/squad/event_redact.go`. Pattern match returns `"<redacted>"` and
  `redacted=true`; truncation to `cfg.MaxLen` bytes (zero-or-negative =
  unlimited at the helper level); pattern wins over truncation.
- [x] `resolveRedactConfig(config.EventsConfig) redactConfig` in the same
  file. Priority: `SQUAD_REDACT_REGEX` env > `events.redact_regex` yaml >
  no pattern. Bad regex at any tier silently falls through (fail-open).
  Yaml `max_target_len` of 0 (or unset) maps to the 200-byte default at
  the resolver layer.
- [x] `cmd/squad/event.go` `runEventRecord` calls
  `redact(target, resolveRedactConfig(cfg.Events))` before passing to
  `RecordEvent`. `discoverRepoRoot`/`config.Load` failures degrade to raw
  passthrough (fail-open hook semantics).
- [x] New `EventsConfig` block on `config.Config` (yaml tag `events`) with
  `RedactRegex` (yaml `redact_regex`) and `MaxTargetLen`
  (yaml `max_target_len`).
- [x] 12 unit tests in `cmd/squad/event_redact_test.go` covering all five
  required scenarios plus pattern-beats-truncation, nil-pattern no-op,
  bad-regex fall-through, both-bad no-pattern, and zero-MaxLen at the
  helper layer.
- [x] New `docs/reference/agent-events-privacy.md` with the four required
  elements: overview of `agent_events` columns, redaction-config table,
  explicit do-not-expose warning, pointer to the dashboard auth posture
  in `api.md`.
- [x] `go test ./cmd/squad/ -count=1 -race` passes — see Resolution
  evidence.

## Notes

- `evidence_required: []` because this is a chore that DOES touch Go code — but the test coverage is already in scope (`event_redact_test.go`). The Go code path is tested; the docs file is verified by reading.
- Defaults are deliberately conservative (truncate to 200, no regex). Operators who want stricter redaction set their own regex.
- If the redaction fires on every event, that's a sign the regex is too broad — log to stderr (not chat, hooks must stay quiet) so the operator can spot it without flooding chat.
- Don't redact `--tool`, `--kind`, `--exit`, etc. — only `--target` carries arg payloads.

## Resolution

### Fix

`cmd/squad/event_redact.go` (new) — `redact(s, cfg)` and
`resolveRedactConfig(cfg)` plus the `redactConfig` struct and
`defaultMaxTargetLen = 200` constant. Pattern is the privacy gate; truncation
is the fallback bound. A regex that fails to compile at either tier is
silently ignored — the recorder is fail-open by design and a hook that
crashed the agent would be worse than one that records too much.

`cmd/squad/event.go` — `runEventRecord` resolves the per-repo
`config.EventsConfig`, calls `redact()` on `target`, then hands off to
`RecordEvent`. A `discoverRepoRoot` or `config.Load` failure degrades to
raw passthrough rather than blocking the hook.

`internal/config/config.go` — added `EventsConfig` (yaml `events`) with
`RedactRegex` and `MaxTargetLen` fields, threaded onto `Config`. The
default-zero ergonomics match other config blocks: an unset value falls
back to the documented default.

`docs/reference/agent-events-privacy.md` (new) — overview of what
`agent_events` records, redaction-config table covering the three layered
sources, explicit do-not-expose warning, and a pointer to the dashboard
auth posture in `api.md`.

### Reviewer findings addressed

The code-reviewer pass flagged two real should-fix items, both fixed
before commit:

1. **`max_target_len: 0` doc/code mismatch.** The doc originally said `0`
   disables truncation; the resolver actually treats it as "use default
   200." Updated the doc to describe the implemented semantics
   (`max_target_len: 0` falls back to 200; set a generous integer to
   effectively disable). The low-level helper still honours 0 as
   unlimited internally — useful for tests, no caller can reach it from
   YAML.
2. **Runes-vs-bytes comment.** `redact()` slices on bytes, not runes.
   Updated the doc-comment to say bytes and noted the worst case is one
   mojibake codepoint at the cap. Targets are paths and short tool args
   in practice.

Two nits acknowledged and not addressed (both flagged for later):

- A stderr line on `discoverRepoRoot` failure inside `runEventRecord` —
  worth doing but the parent `RunE` already swallows the recorder error
  on the same fail-open principle, and the cost-benefit is low for now.
- Per-call regex compile in `resolveRedactConfig` — fine at current
  event volume; revisit if recording becomes interactive.

### Evidence

```
$ go test ./cmd/squad/ -run "TestRedact|TestResolveRedactConfig" -count=1 -v
... 12 tests ...
PASS
ok  	github.com/zsiec/squad/cmd/squad
```

Full race suite: `go test ./cmd/squad/ -count=1 -race` →
`ok  github.com/zsiec/squad/cmd/squad  58.202s`.

Lint: `golangci-lint run` → `0 issues`.

Attestation hash: `dfb36fc6991a46e0333002468b7b3f346466bebf85b8e06e2ff8d441dab924ba`.
