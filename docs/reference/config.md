# Configuration

`.squad/config.yaml` lives at the repo root, is committed to git, and every knob has a default. The file only needs the overrides you actually want; delete any block and squad falls back to the default.

This file is **per-repo**. Operational state (claims, chat, touches) lives in `~/.squad/global.db` and is machine-local.

> Edit this file directly with your editor (Claude can do that too — *"set the WIP cap to 3 per agent"*). Squad reads it on every command; there's no daemon to restart.

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

agent:
  claim_concurrency: 1

verification:
  pre_commit: []

hygiene:
  stale_claim_minutes: 60
  sweep_on_every_command: true

touch:
  enforcement: warn          # warn | deny — default warn
  enforcement_paths: []      # doublestar globs deny-mode actually blocks on
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

## `agent`

```yaml
agent:
  claim_concurrency: 1   # max items a single agent can hold open at once
```

Set to a large number (e.g. `100`) to effectively disable the cap.

## `verification.pre_commit`

Commands `squad done` will run before closing an item. Each entry has a `cmd` and an optional `evidence` regex squad greps from stdout to confirm the run produced a real result (not a silently-skipped suite). Pass `--skip-verify` to override locally.

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

## `hygiene`

```yaml
hygiene:
  stale_claim_minutes: 60         # claims with no last_touch past this are flagged stale and auto-reclaimed
  sweep_on_every_command: true    # run the auto-sweep goroutine on every DB-touching invocation (debounced)
```

`SQUAD_NO_HYGIENE=1` in the environment disables the sweep for a single invocation regardless of config.

## `touch`

Controls the pre-edit touch-check hook (`pre_edit_touch_check.sh`). When another agent is currently touching the file Claude is about to edit, the hook calls `squad touches policy <file>` and emits a `PreToolUse` decision JSON on stdout that Claude reads.

```yaml
touch:
  enforcement: warn          # warn | deny
  enforcement_paths:         # doublestar globs (only consulted in deny mode)
    - go.mod
    - go.sum
    - "**/*.lock"
    - "internal/store/schema.sql"
```

- `enforcement: warn` (default) — every conflict surfaces as `permissionDecision: "ask"` plus an `additionalContext` line naming the owner agent. Claude pauses and asks the user before proceeding.
- `enforcement: deny` — same warning behavior for most paths, but if the target matches one of the `enforcement_paths` globs the decision becomes `permissionDecision: "deny"` and Claude refuses the edit outright. Pair this with red-flag files (dependency manifests, schema files, lockfiles) where parallel writes are almost always a mistake.

Globs use [doublestar](https://github.com/bmatcuk/doublestar) syntax: `*` matches within one path segment, `**` matches across segments.

## Where the file is generated

`squad init` writes the initial `.squad/config.yaml` from a template that picks reasonable defaults based on the project's primary language (Go / Node / Rust / Python). Re-running `squad init` doesn't overwrite the existing config.
