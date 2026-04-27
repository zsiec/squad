# Recipe: Refining captured items

## Who this is for

You triage the inbox and one of the captured items is *almost* right — the title is fine, the area is set, but the acceptance criteria are vague or the problem statement skips the context a fresh agent would need. You don't want to reject it (the idea is good) and you don't want to accept it as-is (it'll waste a claim). Refinement is the third option: send it back with notes, let an agent edit the markdown, and put it back in the inbox sharper than it left.

## The flow at a glance

```
squad inbox ──► squad refine <ID> --comments "..." ──► (item enters needs-refinement)
                                                              │
                                                              ▼
                                          squad refine ──► squad claim <ID> ──► edit ──► squad recapture <ID>
                                                                                                │
                                                                                                ▼
                                                                                          back in inbox
```

Refinement is documentation work. No TDD, no evidence gates, no `squad done` — `recapture` is the close-out.

## Reviewer flow

### From the dashboard

Open the inbox modal, click **Details** on the captured item to read it, click **Refine**, type your comments, **Send**. The item flips to `needs-refinement` and disappears from the regular inbox view.

### From the CLI

```bash
squad refine FEAT-014 --comments "tighten AC: which endpoints, which error codes? Cite the existing handler at api/export.go."
```

The `--comments` flag is required when an id is given. Be specific — the refining agent only sees what you write here.

## Refining-agent flow

Two paths converge on the same outcome — an item that's back in the inbox sharper than it left:

- **Manual edit (this recipe):** claim, $EDITOR the markdown, run `squad recapture`. Best for touch-ups — sharpening AC, adding a file:line, filling in the problem statement.
- **Structured interview:** from a Claude Code session, run `/squad:squad-intake FEAT-014`. The intake skill opens a refine-mode interview, walks the changes question-by-question, drafts a superseding item, and commits — the original is archived (status: superseded) and a fresh id replaces it. Best when the item needs more than a touch-up: rewriting AC, restructuring scope, or splitting into multiple items.

The rest of this recipe walks the manual path.

### 1. List what's waiting

```bash
squad refine
```

With no arguments, `squad refine` lists items in `needs-refinement` (id, age, who captured it, title). These are the items waiting for an editor.

### 2. Claim and read

```bash
squad claim FEAT-014 --intent "address reviewer feedback on AC tightness"
```

The claim ledger is the only correctness contract here — hold the claim while editing so two agents do not refine the same item. There is no code to test, no AC to verify against; the contract is the reviewer's note.

Open the file and read the `## Reviewer feedback` section at the top of the body. That is the brief.

### 3. Edit the markdown

```bash
$EDITOR .squad/items/FEAT-014-wire-the-export-button.md
```

Address the feedback in the body. Sharpen the AC, add file:line references, fill in the problem statement — whatever the reviewer asked for. Leave the `## Reviewer feedback` section in place; `recapture` moves it for you.

### 4. Recapture

```bash
squad recapture FEAT-014
```

This does the bookkeeping in one shot:

- Moves `## Reviewer feedback` into `## Refinement history` as `### Round N — YYYY-MM-DD`, preserving the audit trail across rounds.
- Flips status from `needs-refinement` back to `captured`.
- Releases your claim.

The item reappears in the regular inbox. The reviewer can accept it, reject it, or send it back for another round if the edits did not land.

## Round-tripping

Each refinement round is appended under `## Refinement history`:

```markdown
## Refinement history
### Round 1 — 2026-04-20
tighten AC: which endpoints, which error codes?

### Round 2 — 2026-04-24
non-goals are still ambiguous; mark the bulk-export path explicitly out of scope.
```

The history grows with each round. Reviewers reading the file later see the full back-and-forth without digging through chat.

## A note on scope

Refinement is for sharpening an item, not redesigning it. If the reviewer's feedback amounts to "this should be three items, not one," reject and re-file rather than refine. If it amounts to "the spec is wrong," fix the spec and re-decompose. Refinement is best when the bones are right and the prose needs work.

## See also

- [recipes/triage.md](triage.md) — accept/reject the inbox.
- [recipes/decomposition.md](decomposition.md) — bulk-filing items from a spec.
