# Skills

The squad Claude Code plugin ships nine skills. Each is a Markdown file under `~/.claude/plugins/squad/skills/` after `squad install-plugin`. Claude auto-loads a skill when its description matches the work at hand.

To install all nine: `squad install-plugin`. To remove: `squad install-plugin --uninstall`.

## `squad-loop`

**Triggers:** starting a session, resuming after a pause, "just start coding" instincts.

The operating loop every session follows: register → tick → pick → claim → work → checkpoint → test → review → commit → done. Cross-references `squad-premise-validation`, `squad-evidence-requirement`, and `squad-code-review-mandatory` at the relevant steps. Read in full in [concepts/the-loop.md](../concepts/the-loop.md).

## `squad-premise-validation`

**Triggers:** claiming a BUG item whose AC names a concrete failure ("returns wrong value for X", "breaks when Y").

The rule: write the failing test FIRST against current code. If it passes unmodified, the bug doesn't reproduce — reclassify (BUG → DEBT) instead of "fixing" code that already works. The cost is one test invocation; the alternative is two hours of wrong-direction work.

## `squad-evidence-requirement`

**Triggers:** about to write "tests pass," "build is clean," "this works," or running `squad done`.

Paste the actual command output line — never paraphrase. Bare assertions are unverifiable; a future agent reading your conversation cannot distinguish "I ran the tests" from "I think they would pass." The paste is also a forcing function on you: if you can't paste a green line, you haven't actually verified.

## `squad-code-review-mandatory`

**Triggers:** before every commit on a claimed item, including one-line fixes.

Self-review catches the obvious. An independent reviewer catches the rest. Spawn `superpowers:code-reviewer` with a self-contained briefing including the item file path, the diff, and a "premise-validation latitude" clause permitting the reviewer to revert and verify. Verify each finding before agreeing — performative agreement produces worse code than no review.

## `squad-chat-cadence`

**Triggers:** about to go silent for >20 min on a claim, or about to post to chat.

Picks the right verb: `thinking` (forming a plan), `milestone` (AC done), `stuck` (need help), `fyi` (heads-up), `ask @agent` (directed). Routes to the active claim's thread by default; `--thread global` for cross-agent. Read in full in [concepts/chat-cadence.md](../concepts/chat-cadence.md).

## `squad-quality-bar`

**Triggers:** about to run `git commit`.

The pre-commit checklist: no commented-out code, no TODOs, no defensive checks for impossible states, minimum comments, no PM-trace IDs in code, no premature abstraction, no half-finished implementations, AC literally checked off. If you would not defend the code in review, do not ship it.

## `squad-time-boxing`

**Triggers:** investigations, debugging, "figure out why X is slow."

Default exploration cap: 2 hours. At the cap, stop and write up what you tried, what was ruled out, what's still unknown. Then escalate, parallelize, or `squad blocked`. Long unsuccessful sessions correlate with going down the wrong path; the cap forces a re-evaluation before sunk cost dominates.

## `squad-parallel-dispatch`

**Triggers:** starting a second item in a session, or the user asks for multiple things at once.

Three independent items in three subsystems → dispatch in parallel via `superpowers:dispatching-parallel-agents`. Children run scoped tests; the parent runs the full suite as the integration gate after cherry-picking. Skipping the parent integration gate is how cross-package races ship to main.

## `squad-handoff`

**Triggers:** end of session, pausing mid-item, true handoff to another agent.

The 3-bullet brief: shipped (what closed), in flight (what's open), surprised by (anything not in commits). Mid-flight pause adds a `## Session log` entry to the item file with what you did, what's left, and the next concrete step. True handoff: `squad release <ID> --outcome released` plus the next-step in the chat message.
