# Troubleshooting

Each entry is **symptom → cause → fix.**

## Stale claim blocks me from claiming

**Symptom:** `squad claim FEAT-001` exits with "already claimed by agent-blue" but agent-blue hasn't been online for hours.

**Cause:** agent-blue's session died without releasing. The claim is still live in the DB.

**Fix:**

```bash
squad doctor                                              # confirms the claim is stale
squad ask @agent-blue "stealing FEAT-001, ok?"            # courtesy ping
squad force-release FEAT-001 --reason "agent-blue offline >2h"
squad claim FEAT-001 --intent "..."
```

If agent-blue isn't reachable, skip the `ask` and proceed.

## Plugin not loading in Claude Code

**Symptom:** `/work`, `/pick`, etc. don't autocomplete in Claude Code.

**Cause:** Plugin directory absent or stale, or Claude Code wasn't restarted after install.

**Fix:**

```bash
ls ~/.claude/plugins/squad/                      # should list plugin.json + skills/ + commands/
squad install-plugin                             # re-installs idempotently
# Restart Claude Code
```

## Hook not firing

**Symptom:** SessionStart doesn't auto-register, or pre-commit-tick doesn't block stale commits.

**Cause:** `~/.claude/settings.json` hooks block missing or malformed.

**Fix:**

```bash
squad install-hooks --status                     # what's actually installed
squad install-hooks --yes                        # re-installs the defaults
# Inspect manually:
cat ~/.claude/settings.json | jq .hooks
```

If a hook is in settings.json but still doesn't fire, the script at `~/.squad/hooks/<name>.sh` might be missing or non-executable:

```bash
ls -la ~/.squad/hooks/                           # all five .sh files, executable
squad install-hooks --yes                        # re-materializes the scripts
```

## DB integrity error

**Symptom:** Any squad command exits with a SQLite error mentioning corruption or WAL replay.

**Cause:** Disk filled up mid-write, or the process was killed during a transaction.

**Fix:**

```bash
sqlite3 ~/.squad/global.db "PRAGMA integrity_check"
# If "ok", you're done. If not, recover:
mv ~/.squad/global.db ~/.squad/global.db.broken
sqlite3 ~/.squad/global.db.broken ".recover" | sqlite3 ~/.squad/global.db
```

The `.recover` path reconstructs as much as SQLite can. Operational state is machine-local; you'll lose any unrecoverable rows but not item content (that's in git).

## settings.json got mangled

**Symptom:** `squad install-hooks` or Claude Code reports JSON parse errors on `~/.claude/settings.json`.

**Cause:** Manual edit collided with squad's atomic write, or a non-squad tool wrote bad JSON.

**Fix:** Squad writes to `~/.claude/settings.json.tmp` then renames atomically. If the rename succeeded but the source was bad, restore from a backup:

```bash
ls ~/.claude/settings.json*                      # any .bak files?
# Or rewrite from scratch:
echo '{}' > ~/.claude/settings.json
squad install-hooks --yes                        # rewrites from squad's side
# Then re-add any non-squad hooks you had.
```

## `squad init` refuses to run

**Symptom:** `squad init` exits with "managed block markers in unexpected state."

**Cause:** A previous run was interrupted, or you manually edited the markers in CLAUDE.md.

**Fix:** Open CLAUDE.md and find the squad-managed markers:

```markdown
<!-- squad-managed:start -->
...
<!-- squad-managed:end -->
```

Make sure exactly one start and one end exist, in that order. Delete the entire block (markers included) if you want a clean re-init, then re-run `squad init`.

## Touch warnings won't go away

**Symptom:** `squad touch` keeps warning about a peer's touch even though the peer says they're done.

**Cause:** Peer didn't run `squad untouch` (or the corresponding `squad done` / `squad release`).

**Fix:**

```bash
squad doctor                                     # lists orphan touches; auto-releases them
```

If doctor doesn't auto-release a touch (because it's still attached to an active claim), the issue is the upstream stale claim; force-release per "Stale claim" above.

## Workspace next ignores a repo

**Symptom:** `squad workspace next` doesn't show items from a repo you know exists.

**Cause:** Repo not registered in the global DB. Registration is lazy — happens when any squad command runs in that repo.

**Fix:**

```bash
cd /path/to/the/repo && squad next
# Now the repo is registered; back wherever you were:
squad workspace list                             # confirm it's listed now
```

If it still doesn't appear, the repo_id might be different from what you expect. Check:

```bash
sqlite3 ~/.squad/global.db "SELECT id, root_path, name FROM repos"
```

## Web UI not updating

**Symptom:** `squad serve` dashboard goes stale; new claims/messages don't appear.

**Cause:** SSE connection dropped (browser tab backgrounded, network hiccup).

**Fix:** Reload the tab. The dashboard reconnects on page load.

If the server itself died, restart `squad serve`. Logs go to stderr; if it's exiting on startup, check for port conflicts.

## When all else fails

Capture the diagnostics and file an issue:

```bash
squad version
squad doctor
go env | head -10
sw_vers 2>/dev/null || lsb_release -a 2>/dev/null    # OS info
```

File at https://github.com/zsiec/squad/issues with the output. Include the exact command that failed and the full output.
