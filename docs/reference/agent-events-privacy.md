# Agent events — privacy and redaction

## What's recorded

Each PreToolUse, PostToolUse, SubagentStart, and SubagentStop hook invocation records one row into the global `agent_events` table at `~/.squad/global.db`. A row carries: timestamp, agent id, session id, event kind, tool name, a `target` string lifted from the tool input (typically a path or short argument), exit code, and duration. Bodies, full command lines, file contents, and tool outputs are **not** recorded — only the metadata above. The activity-stream endpoints (`GET /api/agents/{id}/events` and `/timeline`) read this table to render the per-agent drawer in the dashboard SPA.

## Redacting `target`

The `target` field is the only column that can carry caller-supplied text, so it is the column subject to operator-controlled redaction before insert. Two knobs, layered in priority order:

| Source | Field | Default |
|---|---|---|
| Env var | `SQUAD_REDACT_REGEX` | unset |
| `.squad/config.yaml` | `events.redact_regex` | empty |
| `.squad/config.yaml` | `events.max_target_len` | 200 |

If the resolved regex matches `target`, the column stores the literal string `<redacted>` and the per-row metadata is preserved (kind, tool, timestamp). Otherwise the value is truncated to `max_target_len` bytes. `max_target_len: 0` (or unset) falls back to the default of 200, which keeps inadvertent secrets bounded; to effectively disable truncation, set a generous number (e.g. `1000000`). A regex that fails to compile at any tier is silently ignored — the recorder is fail-open by design and a hook that crashed the agent would be worse than one that records too much.

Example config snippet:

```yaml
events:
  redact_regex: '(?i)password|token|secret|api[-_]?key|bearer'
  max_target_len: 200
```

`SQUAD_REDACT_REGEX=(?i)password|...` overrides at runtime without editing the file (useful in CI or shared workstations).

## Do not expose this table publicly

The `agent_events` rows are intended for the operator and the agents collaborating in this repo. They reveal which tools an agent invoked, against which paths, when, and at what cadence — sensitive operational telemetry even after redaction. **Do not** point a public URL at the squad dashboard, ship rows to a third-party log aggregator without separate review, or paste raw rows into a public PR description. The dashboard's posture is loopback-only by default; see [api.md — auth posture](api.md) for the token model and binding rules. If you need to share an event trace with a peer, redact the row by hand or share a screenshot, not the raw JSON.
