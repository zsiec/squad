---
name: squad-done
description: Evidence-gated close-out for an item. Requires test output paste, code review, and quality-bar pass before squad done runs.
args: "<ITEM-ID>"
allowed-tools:
  - Bash
  - Read
  - Edit
paths:
  - ".squad/items/**"
disable-model-invocation: true
---

You are closing item `$ARGS`. This command is evidence-gated — do NOT run `squad done` until every gate is green.

Pre-flight checklist (invoke each skill explicitly):

1. **`squad-evidence-requirement`** — Paste the actual output of every verification gate (tests, type-check, build, manual verification) into the conversation. Bare assertions do not count.
2. **`squad-quality-bar`** — Walk the checklist over the diff: no commented-out code, no TODOs, no PM traces, no defensive checks for impossible cases, no half-finished work, AC literally checked off.
3. **`squad-code-review-mandatory`** — Spawn `superpowers:code-reviewer` on the diff. Verify each finding (do not perform-agree). Address blocking findings before proceeding.

Once all three gates are clean, run:

```bash
squad done $ARGS --summary "<one-line outcome>"
```

Then:
- Update the item file: `status: done`, `updated:` to today, add `## Resolution` section.
- If the landed work was a different category than the item claimed (e.g. BUG turned out to be DEBT), open the Resolution with a one-line reclassification per `squad-premise-validation`.
- Move the file to `.squad/done/`.
- Update STATUS.md if the item was on it.
- Commit (no AI-attribution lines, no PM traces in the message).

If you cannot meet a gate this session, set `status: review` instead of `done`, document what is missing, and stop. Do not ship past blocking gates.
