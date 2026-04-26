---
name: squad-review
description: Spawn superpowers:code-reviewer on an item with a self-contained briefing. Includes premise-validation latitude and working-tree hygiene clauses.
argument-hint: "<ITEM-ID>"
allowed-tools:
  - Bash
  - Read
  - Task
paths:
  - ".squad/items/**"
disable-model-invocation: true
---

You are requesting code review for item `$ARGS`. Invoke the `squad-code-review-mandatory` skill to construct the briefing correctly, then read `superpowers:requesting-code-review` to brief the reviewer.

First, post the request to the item thread:

```bash
squad review-request $ARGS
```

Then spawn the reviewer with a self-contained briefing. The reviewer does not see this conversation; the prompt must include:

- **Item file path:** `.squad/items/$ARGS-*.md` — the `## Acceptance criteria` is the contract.
- **The diff:** paste `git diff main...HEAD` inline if small, otherwise give the command.
- **Specific concerns:** any area you want them to focus on.
- **Output format:** prioritized findings (Critical / High / Medium / Low) with file:line and suggested fix per finding.
- **Premise-validation latitude:** "If the claimed failure seems dubious, empirically verify against the pre-fix code (revert the fix, rerun tests, observe). Report back if the bug does not reproduce so we can reclassify."
- **Working-tree hygiene:** "If you patch/revert/modify any file to verify behavior, restore it. Do not leave `.bak` files, scratch test files, or uncommitted edits behind."

When the reviewer returns, use `superpowers:receiving-code-review` to evaluate each finding. Do not perform-agree. Verify each one, push back on the wrong ones with evidence, file out-of-scope findings as fresh items.
