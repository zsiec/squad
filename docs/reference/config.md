# Configuration

`.squad/config.yaml` lives at the repo root, is committed to git, and every knob has a default. The file only needs the overrides you actually want; delete any block and squad falls back to the default.

This file is **per-repo**. Operational state (claims, chat, touches) lives in `~/.squad/global.db` and is machine-local.

## Top-level structure

```yaml
project_name: <your project>

id_prefixes:
  - BUG
  - FEAT
  - TASK
  - CHORE

defaults:
  priority: P2
  estimate: 1h
  risk: low

risk:
  high:
    paths: []
    extra_requirements: []
  medium:
    paths: []
  low:
    default: true

verification:
  pre_commit: []

chat:
  quiet_nudge_minutes: 20
  default_audience: thread

hygiene:
  stale_claim_minutes: 60
  sweep_on_every_command: true

plugin:
  hooks:
    session_start: on
    pre_commit_tick: off
    pre_commit_pm_traces: off
    pre_edit_touch_check: off
    stop_handoff: off
```

## `project_name`

Display name for the project. Used in dashboards and STATUS.md. Defaults to the directory basename if omitted.

## `id_prefixes`

ID prefixes squad accepts on `squad new <prefix>` and `squad claim`. The defaults work for most projects. Add your own (e.g. `INFRA`, `SEC`, `PERF`) and squad will treat them as first-class types.

```yaml
id_prefixes:
  - BUG
  - FEAT
  - TASK
  - CHORE
  - DEBT
  - INFRA
```

## `defaults`

Default `priority` (P0–P3), `estimate` (`30m`, `1h`, `2h`, `4h`, `1d`, etc.), and `risk` (`low` / `medium` / `high`) applied to newly-filed items if `squad new` doesn't pass overrides.

## `risk`

Risk classification by file path. Items whose acceptance criteria mention paths in `risk.high.paths` (configurable matcher) inherit that risk bucket unless explicitly overridden on the item itself.

```yaml
risk:
  high:
    paths:
      - internal/store/migrations/
      - schema/
    extra_requirements:
      - rollback_plan_in_item
      - effort_max
  medium:
    paths:
      - internal/server/
  low:
    default: true
```

`extra_requirements` is informational — `squad new` won't refuse to file an item missing a rollback plan, but the doctor sweep can be wired to flag it.

## `verification.pre_commit`

Commands `squad done` will run before closing an item. Each entry has a `cmd` and an optional `evidence` regex squad greps from stdout to confirm the run produced a real result (not a silently-skipped suite).

```yaml
verification:
  pre_commit:
    - cmd: "go test ./... -race"
      evidence: "ok\\s.*\\s[0-9.]+s"
    - cmd: "go vet ./..."
      evidence: ""
    - cmd: "golangci-lint run"
      evidence: ""
```

The Phase 11 pre-commit-tick hook can be wired alongside this to enforce on commit, not just on `squad done`.

## `chat`

```yaml
chat:
  quiet_nudge_minutes: 20         # 0 disables; otherwise the hygiene sweep posts a 'nudge' if you hold a claim and haven't ticked for this long
  default_audience: thread        # 'thread' (the active claim) or 'global'
```

## `hygiene`

```yaml
hygiene:
  stale_claim_minutes: 60         # claims with no last_touch past this are flagged stale
  sweep_on_every_command: true    # run the auto-sweep goroutine on every DB-touching invocation (debounced)
```

## `plugin.hooks`

Per-hook default. The actual install state is what `squad install-hooks` wrote to `~/.claude/settings.json`; this block tells `squad install-hooks` what to do when invoked with no `--<hook-name>` flags. See [hooks reference](hooks.md) for descriptions.

```yaml
plugin:
  hooks:
    session_start: on
    pre_commit_tick: off
    pre_commit_pm_traces: off
    pre_edit_touch_check: off
    stop_handoff: off
```

## Where the file is generated

`squad init` writes the initial `.squad/config.yaml` from a template that picks reasonable defaults based on the project's primary language (Go / Node / Rust / Python). Re-running `squad init` doesn't overwrite the existing config.
