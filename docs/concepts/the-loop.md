# The loop

## TL;DR

Every session — yours, every coworker's, every parallel agent's — follows the same eight steps:

> go → pick → claim → work → checkpoint → test → review → done

Skip a step and you produce work that other people have to clean up.

## The eight stages

### 1. Go

`squad go` registers a session-derived agent id, claims the top ready item, prints its acceptance criteria, and flushes any unread chat into your context — in one command. Idempotent: re-run to resume the same claim. If `squad go` says `no ready items`, drop to step 2 and pick manually. Chat after that point is delivered continuously by hooks, so you do not need to manually tick mid-session.

### 2. Pick

Continue an in-progress claim if you have one. Otherwise: `squad next` lists the top of the ready stack. Pre-flight check before claiming: every entry in the item's `blocked-by:` must already be done. If the item references a file:line, open the file and confirm the condition still holds — items rot.

### 3. Claim atomically

`squad claim <ID> --intent "one-sentence plan"`. Exit 0 → you hold it. Exit non-zero → someone else does; pick another. The DB is the live lock; the item file's `status:` field updates at close-out, not now.

### 4. Work

Read the item end-to-end. Verify every `file:line` reference. If the AC names a concrete failing scenario, **write the RED test FIRST** against current code (see [premise validation](#premise-validation)). TDD is the default, not a suggestion.

### 5. Checkpoint

At every meaningful chunk — new file, new test, new abstraction, ~30–60 minutes of focused work — pause and re-read the AC. Has scope crept? Is the diff still the smallest possible? Post `squad milestone "AC 1 green"` so peers see progress without DM-ing.

### 6. Test

Scoped tests during iteration; full suite once before commit. **Paste the actual output line into your conversation** (`ok ./...`, `PASS`, etc.). Bare assertions like "tests pass" are worth zero — a future agent reading your conversation can't tell apart "I ran them" from "I think they would pass."

### 7. Review

Every item goes through `superpowers:code-reviewer` (or your project's equivalent). Yes, even a one-line fix. The agent is cheap; production bugs are not. Verify each finding before silently agreeing — performative agreement produces worse code than no review.

### 8. Done

`squad done <ID> --summary "one-line outcome"`. The command rewrites the item's frontmatter (`status: done`), moves the file to `.squad/done/`, and posts a system message in the item thread. Commit the file move with the rest of the work. Chat is delivered continuously by hooks, so no manual tick is needed before close-out — if you suspect a hook miss, run `squad tick` as a diagnostic.

## Premise validation

Every BUG item carries a premise: "this code is wrong, here is the symptom." The premise is a claim, not a fact. Validate it before you spend two hours fixing nothing:

1. Write the failing test first against the current code.
2. Run it.
   - **Fails for the reason described** → the bug is real. Implement the fix; the test becomes the regression test that lands in the same commit.
   - **Passes unmodified** → the bug doesn't reproduce. Stop. Do not implement a "fix." Reclassify (BUG → DEBT) or close with "no repro."

Items rot. A symptom described two weeks ago might be fixed by an unrelated commit; a line number might point at different code now. Premise validation is a 30-second test invocation that prevents a 2-hour wrong-direction session.

## When the loop bends

- **Tiny one-offs** (a typo fix, a comment cleanup) can skip the loop. Use judgment — if filing the item would take longer than the fix, just commit.
- **Exploratory work** is time-boxed: 2 hours of focused investigation. At the cap, write up what you tried and what's still unknown, then escalate, parallelize, or `squad blocked` — do not silently extend.
- **`squad blocked`** is the right move when you're waiting on an external dependency. Don't sit on a blocked item; release it and pick another.

## Anti-patterns

- **Skipping `squad go`** at session start. Continuous chat hooks need a registered identity; without `squad go` you may post under a stale or missing id.
- **Claim-and-walk-away.** Heartbeat handles brief absence; if you're truly done for the day, `handoff` or release. The hygiene sweep auto-flags claims with no activity past the configured threshold.
- **Test after impl.** Even if the impl works, you've lost the proof that the bug existed — and you've skipped the chance to discover the bug doesn't reproduce.
- **Marking done without re-reading the AC.** Each box should map to a test you wrote or a step you actually performed.

## Why the loop works

It's a forcing function for visibility and accountability. `squad go` keeps the team unblocked — register, claim, mailbox flush in one shot. Atomic claim prevents two agents from doing the same work. Premise validation prevents fixing nothing. Evidence-gated done means the next session can trust your "done." Mandatory review catches what self-review misses. The eight steps together are how a squad of agents actually finishes things instead of meandering.
