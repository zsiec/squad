# Hooks reference

Squad's Claude Code hooks are **opt-in per hook** via `squad install-hooks`. Six default to ON, five default to OFF (opt-in for specific use cases).

## Quick reference

| Hook | Default | Event | Effect |
|---|---|---|---|
| `session-start` | ON | `SessionStart` | Ensures the session has a derived agent id (calls `squad register --no-repo-check`); injects one-line context block. Chat delivery is handled continuously by `user-prompt-tick` plus the `Stop` listen + post-tool-flush mechanism — session-start does not need to surface chat itself. |
| `user-prompt-tick` | ON | `UserPromptSubmit` | Auto-tick before every prompt; injects pending mentions/knocks/handoffs as `additionalContext`. |
| `pre-compact` | ON | `PreCompact` | Inject the agent's current claim + intent + recent chat lines so identity survives compaction. |
| `stop-listen` | ON | `Stop` | Long-block on a localhost TCP listener; wake on any peer say/ask/fyi and inject the inbox before letting the session end. The primary chat-delivery mechanism. |
| `post-tool-flush` | ON | `PostToolUse` | Mailbox flush between tool calls — delivers peer chat as `additionalContext` mid-turn without waiting for `Stop`. ~5ms when inbox empty. |
| `session-end-cleanup` | ON | `SessionEnd` | Drops this session's `notify_endpoints` row so peer senders stop dialing a dead listener port. Necessary companion to `stop-listen`. |
| `async-rewake` | OFF | `asyncRewake` | Background rewake from outside the IDE — wakes an idle session within seconds when a peer needs them. Spawns one background process per session; opt-in until the asyncRewake contract stabilizes. |
| `pre-commit-pm-traces` | OFF | `PreToolUse:Bash` matching `git commit` | Blocks commit if backlog IDs in diff or message. Catches PM noise pre-commit; harmless if you follow the no-PM-traces rule. |
| `pre-edit-touch-check` | OFF | `PreToolUse:Edit\|Write` | Warns (does not block) if peer agent is touching the same file. Useful in multi-agent setups; pure noise solo. |
| `stop-learning-prompt` | OFF | `Stop` | At session end, prompt the agent to file a learning if non-trivial code changed. Adds ~50ms to Stop. Opt in if your team finds learnings worth filing. |
| `loop-pre-bash-tick` | OFF | `PreToolUse:Bash` | Skill-scoped tick — fires only while the squad-loop skill is the active skill. Cheaper than `user-prompt-tick` (Bash-boundaries only) but only fires when the loop skill is loaded. |

## Install

```bash
squad install-hooks                          # interactive, asks per hook
squad install-hooks --yes                    # accept defaults (six hooks ON)
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
