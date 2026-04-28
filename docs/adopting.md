# Adopting squad

This is the cold-start walkthrough. By the end you'll have squad installed, your first item closed, and a feel for the day-to-day rhythm. Total time target: under 5 minutes from install to first done.

The default voice through this doc assumes you're driving squad through Claude Code — that's the recommended path. Each step also shows the terminal-only equivalent for scripts, CI, or anyone who'd rather work in a shell.

## TL;DR

1. Install the `squad` binary: `brew tap zsiec/tap && brew install squad` (or `go install github.com/zsiec/squad/cmd/squad@latest` if you'd rather build from source).
2. From inside Claude Code, add the marketplace and install the plugin:
   - `/plugin marketplace add zsiec/squad`
   - `/plugin install squad@squad`
   - Restart Claude Code (or run `/reload-plugins`) so hooks, skills, and the MCP server load.
3. Open Claude Code in a git repo and ask: *"Initialize squad here and walk me through the example item."*
4. When the example is done, ask: *"Mark it done."*

If you're not using Claude Code, the terminal equivalent is `squad init --yes && squad go` in the project's directory.

## Day 0 — install the binary

Squad is two pieces: a Go binary you run from the shell (`squad ...`) and a Claude Code plugin that exposes the binary's verbs as MCP tools, slash commands, skills, and hooks. The plugin needs the binary, so install the binary first.

The recommended path is Homebrew (macOS, Linux):

```bash
brew tap zsiec/tap
brew install squad
```

If you'd rather build from source — useful for non-tagged commits, or environments without `brew`:

```bash
go install github.com/zsiec/squad/cmd/squad@latest
```

Verify:

```bash
squad version
```

Expected output: `0.2.0` or similar. If you see `squad: command not found`, your `$GOPATH/bin` isn't on `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Persist that in your shell rc.

If you only want the CLI (scripts, CI, no Claude Code integration), you're done — skip to [Day 0 — initialize a repo](#day-0--initialize-a-repo).

## Day 0 — install the Claude Code plugin

Squad ships as a Claude Code marketplace at `github.com/zsiec/squad`. From inside any Claude Code session:

```
/plugin marketplace add zsiec/squad
/plugin install squad@squad
/reload-plugins
```

After `/reload-plugins` (or a Claude Code restart), the plugin's pieces are live:

- **MCP tools** — `squad_claim`, `squad_next`, `squad_done`, `squad_say`, etc. visible in the tool list.
- **Slash commands** — `/squad:work`, `/squad:done`, `/squad:pick`, `/squad:file`, plus the capture-side trio `/squad:squad-capture`, `/squad:squad-intake`, and `/squad:squad-decompose`, all show up under `/help`. Claude Code namespaces plugin commands as `/<plugin>:<command>`; the capture commands kept the `squad-` prefix in their filenames so the doubled `squad:squad-` is intentional.
- **Skills** — squad-loop, squad-handoff, squad-quality-bar, etc. auto-load when their `paths:` match the file you're editing.
- **Hooks** — session-start, user-prompt-submit, stop, pre-compact, etc. fire automatically.
- **Dashboard daemon** — on the first MCP boot, squad installs http://localhost:7777 as a per-user system service (launchd on macOS, systemd-user on Linux). The daemon supervises itself across reboots and binary upgrades; the URL is printed by `squad serve` at startup. Set `SQUAD_NO_AUTO_DAEMON=1` to skip the auto-install and run `squad serve` manually instead.

If you'd rather wire the plugin from a local git checkout (e.g., for plugin development), point Claude Code at the directory directly:

```
claude --plugin-dir ~/dev/squad/plugin
```

The CLI-only path with no Claude Code integration is just `go install` above; no `install-plugin` step is needed.

## Day 0 — initialize a repo

Open Claude Code in your project (`cd ~/dev/your-project` first) and ask:

> *"Initialize squad here."*

Claude runs `squad init --yes` for you. The repo gets the default ID prefixes (`BUG,FEAT,TASK,CHORE`), the example item, and a managed block in `CLAUDE.md`. If you want to customize anything (project name, prefixes), run `squad init` interactively in a terminal instead — it asks ≤3 questions.

What lands on disk:

- `.squad/items/EXAMPLE-001-try-the-loop.md` — a tutorial item that walks the loop with you.
- `.squad/items/`, `.squad/done/` — directories for items you'll file.
- `.squad/config.yaml` — project config; tune later.
- `AGENTS.md` — generic agent doctrine doc.
- `CLAUDE.md` — managed block injected (or file created).
- `.gitignore` — squad lines appended.

Commit it:

```bash
git status
git add .squad/ AGENTS.md CLAUDE.md
git commit -m "chore: adopt squad"
```

The repo must be a git repository (any state — even zero commits is fine). Squad uses `git rev-parse` to derive a stable repo id; without a git repo, you'll get a clear error and `squad init` refuses to run.

## Day 0 — claim and walk through the example

Ask Claude:

> *"Claim the example item and walk me through it."*

Claude calls `squad_next` to find the priority pick (the example item, on a fresh init), `squad_claim` to lock it atomically, prints the acceptance criteria, and flushes any pending peer chat into your context. The example walks you through one full loop with no real work at stake.

(Terminal equivalent: `squad go` does init-if-needed + register + claim + print AC + flush mailbox in one idempotent invocation.)

## Day 0 — close the example

Read the AC, do what it says (post a `squad milestone`, run `squad whoami`, etc. — Claude can do these for you, or run them in a shell yourself). When the AC checks off, ask Claude:

> *"Mark EXAMPLE-001 done with summary 'loop complete'."*

Claude calls `squad_done`. The file moves to `.squad/done/`. Commit:

```bash
git add .squad/
git commit -m "chore: complete first squad loop"
```

That's the whole cycle. Everything else is repetition.

(Terminal: `squad done EXAMPLE-001 --summary "loop complete"`.)

## Day 1 — your first real item

New items land in `captured` state — filed but not yet claimable — so you can capture fast and shape later. Three paths in:

- **Quick capture.** *"Capture a bug for the retry-on-503 panic"* (or `/squad:squad-capture <description>`, or `squad new feat "..."` in a terminal). Claude infers the type and files a frontmatter-only stub.
- **Structured intake.** *"Run intake on my idea about a CSV export"* (or `/squad:squad-intake <starting idea>`). Claude opens an interview, asks one focused question per turn — area, AC, scope, non-goals — drafts a bundle (item, or spec + epics + items if the idea is large), and confirms before committing. Pass an existing item id (`/squad:squad-intake FEAT-007`) to refine an undercooked stub instead of starting fresh; refine-mode commits a superseding item and archives the original.
- **Spec decomposition.** *"Decompose the auth-rework spec into items"* (or `/squad:squad-decompose auth-rework`). Claude reads the spec and drafts 3–7 captured items linked to the parent.

All three land in `squad inbox`, not in `squad next`. Triage from there:

- `squad accept FEAT-001` — runs the Definition of Ready check (area set, ≥1 AC checkbox, real title or problem). Passing items flip to `status: open` and become claimable.
- For sharpening a captured item, use the dashboard's "Send for refinement" button — select passages, attach inline comments, claude redrafts the body. Same flow is available via the MCP `squad_auto_refine_apply` tool.
- `squad reject FEAT-001 --reason "duplicate of FEAT-003"` — deletes the file; the reason is logged to `.squad/inbox/rejected.log`.

For the full triage loop, see [recipes/triage.md](recipes/triage.md). For larger work that decomposes into many items, see [recipes/decomposition.md](recipes/decomposition.md).

## Day 1 — install optional hooks

Ten hooks are on by default after `/plugin install squad@squad` (session-start, user-prompt-tick, pre-compact, stop-listen, post-tool-flush, session-end-cleanup, subagent-start, subagent-stop, task-created, task-completed). Five more are opt-in:

```bash
squad install-hooks
```

Interactive — asks about each. Recommended additions:

- `pre-commit-pm-traces` — Y if you tend to leak ticket IDs into commits.
- `pre-edit-touch-check` — Y if you're going multi-agent.
- `async-rewake`, `stop-learning-prompt`, `loop-pre-bash-tick` — opt in if your team uses those workflows.

See [reference/hooks.md](reference/hooks.md) for what each one does.

## Day 2 — multi-agent

Open a second Claude Code session in the same repo, ideally in a different terminal tab so the `TERM_SESSION_ID` differs. Each session derives a distinct agent id automatically.

In the second session, ask: *"Claim the next ready item."* Atomic SQLite `BEGIN IMMEDIATE` claims mean two sessions can't both grab the same item — exactly one wins, the other gets a clean error. File-touch tracking warns when you're about to edit a file the peer already touched.

If both sessions share the same `TERM_SESSION_ID` (some terminal multiplexers do this), set a unique session var per shell so each derives a distinct agent id:

```bash
export SQUAD_SESSION_ID=blue   # in shell A
export SQUAD_SESSION_ID=red    # in shell B
```

Then run `squad register` (or any squad command — the env var is read on each invocation) inside each shell.

The full multi-agent walkthrough is at [recipes/multi-agent-parallel-claude-sessions.md](recipes/multi-agent-parallel-claude-sessions.md).

## Day 7 — hygiene

Ask Claude:

> *"Run a squad health check."*

Claude calls `squad_status` for the quick view (claimed / ready / blocked / done counts). For the full diagnostic — stale claims, ghost agents, orphan touches, broken refs, DB integrity — run `squad doctor` in a terminal:

```bash
squad doctor                # report findings; exit 0 either way
squad doctor --strict       # exit non-zero if findings exist (CI use)
```

Run it weekly as a habit. If it flags stale claims, the output names the recovery command. See [concepts/hygiene.md](concepts/hygiene.md).

## When things go wrong

See [troubleshooting.md](troubleshooting.md). The fastest path to a fix:

1. Ask Claude to run a health check (`squad_status`) — clears 80% of issues.
2. Run `squad workspace list` in a terminal to confirm the repo is registered.
3. File a bug against squad itself if the issue is a real bug. Your snag is the next person's snag.

## When you graduate

You'll know you've adopted squad when:

- You don't think about the loop anymore — you just describe what you want and Claude does the squad work.
- You file items reflexively, without deliberating.
- `squad doctor` is silent for a week at a time.
- You can't remember what coordinating with peers was like before atomic claims.

That's the success criterion. The loop is invisible when it's working.

## Coming from Claude Code agent-teams?

If you've been using Claude Code's experimental agent-teams and your work is starting to span multiple sessions, days, or machines, you may be ready to migrate. Walk through [recipes/migrating-from-agent-teams.md](recipes/migrating-from-agent-teams.md) — it's the dedicated step-by-step path.

If you're not yet sure whether you've outgrown agent-teams, the [decision matrix](concepts/squad-vs-agent-teams.md) makes the call concrete.

Composing the two is also fine: a squad-managed repo can host a single ephemeral agent-teams session inside one squad claim. Squad items remain authoritative for any work that needs to outlive the agent-teams session.
