# Slash commands

The squad Claude Code plugin ships eight slash commands. Each is a Markdown file under `~/.claude/plugins/squad/commands/` after `squad install-plugin`. Each wraps a binary call and injects framing (when to use the loop, what skill to invoke, etc.) into the conversation.

## `/work`

Onboard or resume in one step. Invokes `squad go`, which inits `.squad/` if absent, registers a session-derived agent id, claims the top ready item, prints its acceptance criteria, and flushes any unread chat into your context. Idempotent — re-running resumes the same claim. The framing then invokes the `squad-loop` skill so subsequent steps follow the discipline.

```
/work
```

## `/pick <ID>`

Atomically claim an item by ID. Verifies references, applies RED-first if AC names concrete failures.

```
/pick FEAT-001
```

Behind the scenes: `squad claim $ARGS --intent "..."`. If the AC names testable failures, the command tells the conversation to invoke `squad-premise-validation` and write the RED test before any implementation.

## `/done <ID>`

Evidence-gated close-out. Walks the conversation through three explicit gates before `squad done` runs:

1. `squad-evidence-requirement` — paste verification output
2. `squad-quality-bar` — run the pre-commit checklist
3. `squad-code-review-mandatory` — spawn a reviewer

Then runs `squad done $ARGS --summary "..."`. If any gate cannot be met this session, the command instructs setting `status: review` instead of done. Chat delivery is continuous via hooks, so no manual tick is needed before close-out.

```
/done FEAT-001
```

## `/handoff`

End-of-session 3-bullet brief plus claim release. Invokes the `squad-handoff` skill, then asks the conversation to post the brief: shipped / in flight / surprised by.

```
/handoff
```

## `/review <ID>`

Spawn `superpowers:code-reviewer` on an item with a self-contained briefing. Includes premise-validation latitude (reviewer may revert and verify) and working-tree hygiene clauses (clean up `.bak` files).

```
/review FEAT-001
```

Posts the request to the item thread via `squad review-request $ARGS` first.

## `/tick`

Diagnostic-only: manually surface mentions and file conflicts. Chat is normally delivered continuously via the `Stop` listen + post-tool-flush + user-prompt-tick hooks, so `/tick` is for cases where you suspect a hook miss or want to advance the read cursor explicitly.

```
/tick
```

Wraps `squad tick`. The framing pushes the agent to actually act on what's surfaced rather than glance and continue.

## `/blocked <ID>`

Mark an item blocked and inject the standard `## Blocker` section template. The template has slots for "what blocks," "what is needed to unblock," and "who/what could unblock."

```
/blocked FEAT-001
```

Wraps `squad blocked $ARGS --reason "..."`. The framing pushes the agent to pick the next item rather than sit on a blocked one.

## `/file <type> "<title>"`

File a new item. Runs `squad new $ARGS`, then injects the standard item template (`## Problem` / `## Context` / `## Acceptance criteria` / `## Notes`) for the agent to fill in.

```
/file bug "race in the cache flusher"
/file feat "add export button"
```

For `risk: high` items where rollback is non-trivial, the framing reminds the agent to add a `## Rollback plan` section before any code lands.
