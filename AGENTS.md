# squad — Agent Operating Manual

How every session goes. Make progress with no ceremony, never break what you wouldn't defend in review.

## §0 — Mental model

- Items: `.squad/items/` (markdown, YAML frontmatter + body). Done → `.squad/done/`.
- Board `.squad/STATUS.md` is a pointer; items are the contract.
- Claims, chat, file touches: `~/.squad/global.db` (machine-local).
- Plans: `docs/plans/` (gitignored).

## §1 — Resume a session

Run `squad go`. It inits, registers, claims the top ready item, prints AC, flushes the mailbox. Idempotent. The plugin's `/work` does the same.

```bash
squad go
```

If it says `no ready items`, ask the user, `squad new`, re-run. If you have an in-progress claim, `squad go` resumes it.

## §2 — Pick an item (if not using `squad go`)

Priority: (1) in-progress claims you hold; (2) newly unblocked items; (3) ready items by priority (P0→P3) then smallest estimate; (4) user asks (file with `squad new` if >1h).

```bash
squad next
squad claim <ID> --intent "one sentence" [--touches path1,path2]
```

`claim` exit 1 means already claimed — pick another. The DB is the live lock.

## §3 — Work the item

1. Read end-to-end. AC is the contract.
2. **Verify every `file:line` reference against current code.** Item bodies rot — fix the item first if stale.
3. **TDD by default.** Failing test → minimal impl → green → commit. Skip only for spikes, fully-covered refactors, or one-offs. Never silently.
4. AC names concrete failures? Write **RED tests first** against unfixed code. If they pass unmodified, reclassify or close no-repro.
5. **Stay chatty.** Post at the cadence moments in §12 — silence on a held claim erases the *why*.

### Anchor checkpoints (≥1d items)

Long items drift. Every meaningful chunk (~30–60 min of focused work), pause and check:

1. **Re-read the item's `## Acceptance criteria`.** Does the current diff progress toward each line, or has scope crept?
2. **Re-read `docs/plans/<id>.md`** (if you wrote one). Is the next concrete step still right?
3. **Did anything new surface that should be a separate item?** File the BUG/CHORE now while it's fresh.
4. **Are you still working on the smallest possible diff that meets the contract?** If sprawling, split: land the minimum first, file the rest as follow-ups.

Mentions, knocks, and file conflicts arrive continuously through hooks — address any that surface before the four questions above. A correction at hour 2 is much cheaper than at hour 6.

## §4 — Test before claiming done

Run verification from `.squad/config.yaml`. Scope iteration by package; full suite once before commit.

**Evidence requirement.** Paste actual output. "Tests pass" without evidence is worth zero — silent pass is indistinguishable from fabrication.

## §5 — Code review (mandatory, every item)

Even one-liners. Spawn `superpowers:code-reviewer` with diff + item file. Verify each finding — don't perform-agree. Push back with evidence when wrong. File follow-up items for out-of-scope findings. Blocking issues open → leave `status: review` and stop.

For external review: `squad review-request <ID> [--mention @reviewer]`. Brief the reviewer with the item file path, the diff, the specific concerns, and an output format (prioritized findings: Critical / High / Medium / Low with file:line).

**Premise-validation latitude.** Tell the reviewer: "If the claimed failure seems dubious, empirically verify against the pre-fix code. If the bug doesn't reproduce, report back so we can reclassify the item."

**Working-tree hygiene.** "If you patch any file to verify behavior, restore it. Do not leave `.bak` files or scratch edits behind."

## §6 — Commit and close

```bash
git add <files>
git commit -m "fix: <concise summary>"   # or feat: / docs: / refactor: / chore:
```

Subject ≤72 chars, lowercase after prefix, imperative. No `Co-Authored-By:` unless explicitly requested. **Never reference item IDs in commit messages.**

```bash
squad done <ID> --summary "one-line outcome"
```

Releases claim, archives item to `.squad/done/`, posts to global + thread, auto-untouches. Update `.squad/STATUS.md` if you were on it.

### Quality bar before commit

Tests passing is necessary but not sufficient. **Don't mark an item done until you'd defend the code in review.** If you're embarrassed by it, refactor it or split the dirty bits into a follow-up CHORE.

Concrete signals to check before committing:

- **No commented-out code.** Delete it. Git remembers.
- **No `TODO`, `FIXME`, or "future work" comments.** If it's worth doing, file an item.
- **No defensive checks for things that can't happen.** Trust internal invariants; only validate at system boundaries.
- **Minimum comments. Default to none.** Only WHY when genuinely non-obvious.
- **No project-management traces in code.** No backlog IDs in filenames, identifiers, comments, or commit messages.
- **No premature abstraction.** Three similar lines beats a wrong abstraction.
- **No half-finished implementations.** End-to-end against the AC, or it's not done.
- **Acceptance criteria literally checked off?** Re-read the item file's `## Acceptance criteria`.

If you genuinely can't meet the bar this session, set `status: review` instead of `done`, write what's wrong in the resolution notes, and file the follow-up.

## §7 — Filing a new item

`squad new <type> "<title>" --priority P[0-3] --area <subsystem>` scaffolds the file. Types: `bug`, `tech-debt`, `feature`, `chore`, `task`.

ID prefixes for this project:
- `BUG-NNN`
- `FEAT-NNN`
- `TASK-NNN`
- `CHORE-NNN`

If P0/P1, add to the Ready section of `.squad/STATUS.md`. Numbers are monotonic per-prefix. Half-baked thoughts? File them anyway with `priority: P3` and `status: open`; triage later.

### Item file template

```markdown
---
id: <PREFIX>-XXX
title: <one-line title>
type: bug | tech-debt | feature | chore | task
priority: P0 | P1 | P2 | P3
area: <subsystem name>
status: open | in_progress | review | blocked | done
estimate: 30min | 1h | 2h | 1d | 3d | 1w | 2w
risk: low | medium | high
created: YYYY-MM-DD
updated: YYYY-MM-DD
references:
  - path/to/file.go:LINE
relates-to: [OTHER-ID]
blocked-by: []
---

## Problem
What is wrong / what doesn't exist. 1–3 sentences.

## Context
Why this matters. Where in the codebase. What's been tried.

## Acceptance criteria
- [ ] Specific, testable thing 1
- [ ] Specific, testable thing 2
- [ ] (For bugs) A failing test exists; it now passes.

## Resolution
(Filled in when status → done.)
```

### Done contracts (per type)

| Type | Done means |
|---|---|
| **bug** | A test reproducing the original bug exists in the repo and now passes. |
| **tech-debt** | No behavior change AND a named metric improved with before/after numbers. |
| **feature** | Acceptance criteria all checked. UI: end-to-end verified. |
| **chore / task** | The thing is done. Verify by running it. |

If you can't write a done-contract assertion for the item, the AC are too vague — sharpen them before coding.

## §8 — Multi-agent dispatch

**Default to parallel.** If 2+ ready items are in different subsystems with no shared state and no dependency between them, dispatch in parallel. Sequencing them out of habit wastes wall-clock time.

Subagents can't see chat — before spawning, the parent must bake any unaddressed mentions or knocks into the sub-brief. Continuous hooks deliver chat to the parent, so the freshest state is whatever is already in your context.

**Do not** dispatch parallel agents for items in the same file/directory (merge conflicts), items where one's findings might change another's approach, or exploratory work (a single focused investigation is faster).

**Test gates are not symmetric.** Children run only their package-scoped tests during TDD. They must NOT run the full suite. The parent is the integration gate; after cherry-picking parallel diffs, the parent runs the full suite **before** dispatching code review.

After agents return, **verify their work.** Read the actual diff before reporting "done." Trust but verify — agent summaries describe intent, not always reality.

## §9 — Handoff between sessions

If a session ends mid-item:

- Leave the item `status: in_progress`.
- Add a `## Session log` section to the item file with: what I did, what's left, gotchas / dead-ends to avoid, next concrete step.
- If you've changed code that isn't yet tested or committed, leave a note in `.squad/STATUS.md` under In Progress with `(uncommitted on branch X)`.
- `squad release <ID> --outcome released` if truly handing off.

### End-of-session brief (every session)

Before signing off, post a 3-bullet summary:

1. **Shipped:** items closed (link by ID + one-line outcome). If nothing closed, say "nothing closed."
2. **In flight / queued:** what's `in_progress` or `review` and where it is.
3. **Surprised by:** anything the next session should know that isn't in an item file or commit message. Skip if genuinely nothing.

## §10 — Escalation / blocked

If you hit something you can't resolve:

- Set `status: blocked`.
- Add `## Blocker` section: what blocks, what's needed, who/what could unblock.
- Move out of In Progress → Blocked in `.squad/STATUS.md`.
- Pick the next item and continue. Don't sit on a blocked item.

## §11 — Time-boxing exploratory work

Some items have unclear scope. These can become black holes. Time-box them: **default exploration cap is 2 hours** of focused work. If you're 2h in and still don't understand the problem, **stop and write up what you know** — hypothesis space tried, ruled-out causes, evidence collected, what's still unknown — then either escalate, spawn a parallel agent on the most promising remaining hypothesis, or set `status: blocked`.

Don't quietly extend the cap. Long unsuccessful sessions are a signal, not a setback.

## §12 — Chat cadence

The backlog is durable; chat is where the team stays in sync while that durable state is being changed. Post often, post small, post honestly. Peers reading later (human or agent) should be able to reconstruct your thinking, not just your commits.

**Verbs.** Use the shortest one that fits. All route to your current claim thread by default; pass `--to <ID>` or `--to global` to override. All accept `@agent` mentions.

| Verb | When |
|---|---|
| `squad thinking <msg>` | Sharing where your head's at — *before* committing, when a plan is still forming. |
| `squad milestone <msg>` | A checkpoint: AC green, phase done, test landing. |
| `squad stuck <msg>` | You're blocked — others can jump in. |
| `squad fyi <msg>` | Heads-up — direction change, surprise, discovery. |
| `squad ask @agent <msg>` | Directed question to one agent. |
| `squad say <msg>` | Plain chat — escape hatch when no verb fits. |

**Cadence.** Post on claim, on direction change, on AC complete, on commit, on surprise, on blocker, on session pause. **Too much?** If the post is just "starting" / "resuming" / "still working" with no new information, cut it. The goal is *visibility into non-obvious state*, not a change log.

**Recognition.** Anchor thanks to the specific behavior (e.g. *"the orphan-ref grep was the catch I'd have missed"*, not *"great work!"*). Generic cheer dilutes the audit log and primes sycophancy in reviewer-agent roles. The `squad-chat-cadence` skill carries the full rule.

## §13 — Anti-patterns (the load-bearing ones)

- **Don't claim "done" without running tests.** Bare assertions are worth zero.
- **Don't ship past blocking review.** Step §5 is mandatory.
- **Don't perform-agree with the reviewer.** Verify each finding.
- **Don't `--no-verify` on commits.** Fix the cause.
- **Don't skip TDD silently.**
- **Don't reference item IDs in commit messages or code.** Backlog lives in `.squad/items/`; code does not.
- **Don't write `TODO` / `FIXME` comments.** If it's worth doing, file an item.
- **Don't add comments that restate code.** Only WHY when non-obvious.
- **Don't grind silently between claim and done.** Silence on a held claim erases the *why* behind the diff and wastes peer wall-clock.
- **Don't open more than one in-progress claim per session** unless using parallel agents.
- **Don't dispatch parallel agents on related work.** They will conflict.
- **Don't extend a time-box silently.** Default exploration cap is 2h.
- **Don't ship without an evidence paste.**

## §14 — When in doubt, ask

Risky actions (deploy, push to main, force-push, edit secrets, drop a database) — **stop and ask**. Once is cheap; an unwanted action is expensive.

Low-risk small calls (file naming, comment placement) — pick a default that matches surrounding code and proceed.

## §15 — Learnings (durable, write-gated)

Cross-item lessons (gotchas, patterns, dead-ends) go in `.squad/learnings/`.
Flow: **propose → human approve → auto-load**.

- `squad learning propose <kind> <slug> --title ... --area ...` writes a
  stub (`gotcha`, `pattern`, `dead-end`); fill its headers.
- `squad learning approve <slug>` synthesizes approved entries into
  `.claude/skills/squad-learnings.md`, auto-loaded on matching paths.
- `squad learning reject <slug> --reason ...` archives under `rejected/`.

Agents don't write `AGENTS.md` directly. Propose a diff:
`squad learning agents-md-suggest --diff <file> --rationale ...`. A human
approves or rejects — LLM-curated AGENTS.md degrades success rates.

---

*Generated by `squad init` from squad's bundled `AGENTS.md.tmpl`. Edit to match your project. The doctrine is portable; the specifics are yours.*
