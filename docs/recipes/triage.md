# Recipe: Triaging the inbox

## Who this is for

You file ideas fast — sometimes mid-conversation, sometimes between commits, sometimes in a slash command — and you want a deliberate moment later to decide which of those ideas actually belong in the next-up stack. This is the capture-then-promote loop.

## The flow at a glance

```
squad new (captured) ──► squad inbox ──► squad accept (open, claimable)
                                    └──► squad reject --reason "..."
```

Captured items are filed but not eligible for `squad next`. Acceptance runs the Definition of Ready check; rejection is permanent and requires a reason.

## Walkthrough

### 1. File a few captured items

```bash
squad new feat "wire the export button"
squad new bug "race in the cache flusher"
squad new chore "rotate API keys before quarter end"
```

Each command writes a frontmatter-only stub under `.squad/items/`, with `status: captured` and `captured_by` / `captured_at` set automatically. None of them show up in `squad next` yet.

### 2. List the inbox

```bash
squad inbox
```

You'll see the three items above, with their kind, title, and DoR status (most will fail at least one rule because they have no AC and no area set).

Useful filters:

```bash
squad inbox --mine               # only items you captured
squad inbox --ready-only         # only items that already pass DoR
squad inbox --rejected           # log of rejected items (separate flow)
```

### 3. Try to accept one

```bash
squad accept FEAT-001
```

This will fail. The output names the violations:

```
FEAT-001 not ready:
  - area-set: area is unset or <fill-in>; set it to a real value
  - acceptance-criterion: no acceptance criteria checkbox; add at least one
    '- [ ] ...' line under '## Acceptance criteria'
```

This is by design. Acceptance is the moment you commit to the work being shaped enough for someone to pick up.

### 4. Edit the file to fix the violations

```bash
$EDITOR .squad/items/FEAT-001-wire-the-export-button.md
```

Set `area:` to something real (e.g. `frontend`, `api`, `infra`), and add an `## Acceptance criteria` section with at least one checkbox:

```markdown
## Acceptance criteria

- [ ] /api/export returns 200 with a CSV body for the happy path
- [ ] error case (no rows) returns 204
- [ ] button in the toolbar is wired to /api/export and shows a loading spinner
```

Optional: re-check without committing to acceptance.

```bash
squad ready --check FEAT-001
# OK
```

### 5. Accept

```bash
squad accept FEAT-001
```

The frontmatter rewrites to `status: open`, `accepted_by` and `accepted_at` get set, and the item now appears in `squad next`. Anyone can claim it.

### 6. Reject one with a reason

```bash
squad reject BUG-001 --reason "duplicate of BUG-007 from last sprint"
```

The file at `.squad/items/BUG-001-...md` is deleted. A row is appended to `.squad/inbox/rejected.log` with the id, title, reason, agent id, and timestamp.

The `--reason` flag is required; there's no way to reject silently.

### 7. Review what got rejected

```bash
squad inbox --rejected
```

Reads the log and prints a summary. Useful when you suspect a reject-loop (the same idea getting filed and rejected repeatedly), or when you're second-guessing a deletion.

There is no un-reject. If a rejected item turns out to matter, file a fresh `squad new`.

## When to triage

Two reasonable cadences:

- **Daily**, for active projects: 5 minutes at the start or end of the day to clear the inbox.
- **When the inbox blocks you**: `squad next` shows nothing because nothing is open; check the inbox first before assuming there's no work.

`squad doctor` flags an inbox that's been stale for more than a week, so you don't have to remember.

## Doing this through Claude Code

The same flow works through MCP tools. From a Claude Code session:

> *"What's in the inbox?"*

Claude calls `squad_inbox` and reads back the list with DoR status.

> *"Accept FEAT-001."*

Claude calls `squad_accept`. If it fails DoR, Claude reports the violations and can offer to draft AC for you to review.

> *"Reject BUG-001 as a duplicate of BUG-007."*

Claude calls `squad_reject` with the reason.

The slash-command shortcut `/squad-capture <kind> "..."` files a captured item from anywhere in the conversation without breaking flow.

## See also

- [concepts/intake.md](../concepts/intake.md) — the model behind capture and acceptance.
- [recipes/decomposition.md](decomposition.md) — bulk-filing items from a spec.
