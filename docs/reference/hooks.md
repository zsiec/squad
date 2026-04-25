# Hooks reference

Squad's Claude Code hooks are **opt-in per hook** via `squad install-hooks`. `session-start`, `user-prompt-tick`, and `pre-compact` are on by default.

## Quick reference

| Hook | Default | Event | Effect |
|---|---|---|---|
| `session-start` | ON | `SessionStart` | Auto `squad register` + `squad tick`; injects one-line context block. |
| `user-prompt-tick` | ON | `UserPromptSubmit` | Auto-tick before every prompt; injects pending mentions/knocks/handoffs as `additionalContext`. |
| `pre-compact` | ON | `PreCompact` | Inject the agent's current claim + intent + recent chat lines so identity survives compaction. |
| `pre-commit-pm-traces` | OFF | `PreToolUse:Bash` matching `git commit` | Blocks commit if backlog IDs in diff or message. |
| `pre-edit-touch-check` | OFF | `PreToolUse:Edit\|Write` | Warns (does not block) if peer agent is touching the same file. |
| `stop-handoff` | OFF | `Stop` | Auto-handoff if open claim with no recent activity (>30 min). |

## Install

```bash
squad install-hooks                          # interactive, asks per hook
squad install-hooks --yes                    # use defaults (session-start + user-prompt-tick + pre-compact ON)
squad install-hooks --yes \
    --pre-commit-pm-traces=on \
    --pre-edit-touch-check=on                # tune individually
```

## Uninstall / status

```bash
squad install-hooks --status                 # what is installed
squad install-hooks --uninstall              # remove all squad-managed entries
```

`--uninstall` removes only entries marked with the squad marker. Any non-squad hooks in your `~/.claude/settings.json` stay untouched.

## Emergency disable

```bash
export SQUAD_NO_HOOKS=1
```

Every squad hook short-circuits to `exit 0` when this is set. Use it if a hook is misbehaving — you will not lose data, the hook just does nothing. Unset to re-enable.

## Where the scripts live

`squad install-hooks` materializes the embedded scripts to `~/.squad/hooks/` and points `~/.claude/settings.json` at them. The scripts are short POSIX shell. Do not edit them in place — they are overwritten on the next `install-hooks` run.

## How to write a custom non-squad hook

Add it to `~/.claude/settings.json` directly. Squad leaves any entry without the `squad` marker alone. Example:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "/usr/local/bin/my-hook.sh" }]
      }
    ]
  }
}
```
