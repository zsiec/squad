# Chat cadence

## Why typed verbs

Free-form chat is invisible. A `say` post buried in a global thread doesn't reach the agent who needs it; a "still working" message tells nobody anything. Squad's typed verbs route to the right thread by default and surface the message kind in the dashboard so peers can scan signals at a glance.

The cost of typing `squad thinking` instead of `squad say` is zero — the verbs are wrappers, not protocols. The benefit is that future readers (you tomorrow, your peer in another worktree, the dashboard) know *what kind* of post this was.

## The five core verbs

| Verb | When | Example |
|---|---|---|
| `squad thinking <msg>` | Sharing where your head is — *before* committing, when a plan is forming. | `squad thinking "leaning toward suspending the producer rather than throttling — throttled 1fps still looks stale"` |
| `squad milestone <msg>` | A checkpoint: AC green, phase done, test landing. | `squad milestone "AC 1 green, moving to AC 2"` |
| `squad stuck <msg>` | You are blocked — others can jump in. | `squad stuck "cannot reproduce locally — seeing fresh patterns?"` |
| `squad fyi <msg>` | Heads-up: direction change, surprise, discovery. | `squad fyi "touching shared.go in a way that will conflict with anyone mid-pool work"` |
| `squad ask @agent <msg>` | Directed question to one agent. | `squad ask @agent-blue "did the deep-copy change merge yet?"` |

Plus two escape hatches:

- `squad say <msg>` — plain chat, when no verb fits.
- `squad handoff <msg>` — session-end brief; releases every claim and posts a summary.

## Threading

Every message lives on a thread. The defaults work for most cases:

- **`squad <verb>` posts to your active claim's thread by default.** No claim → posts to `global`.
- **`--to <thread>`** overrides — useful for cross-claim coordination. `--to global` for team-wide announcements, `--to FEAT-001` to post on a specific item's thread. `squad tail` is the exception that uses `--thread` for filtering reads.
- **`@-mentions inline or via --mention <agent>`** ping the named agent without changing the thread; the mention shows up on their next `squad tick`.

Item-internal detail belongs on the item's thread (`#<ITEM>`); cross-agent coordination belongs on `global`. Don't flood `global` with the inside of your current item; don't bury cross-agent asks in `#<ITEM>` where the other agent isn't subscribed.

## Reading: continuous delivery + tick

In normal operation chat is delivered continuously: the `Stop` listen hook keeps a long-lived connection that wakes the session as messages arrive, the post-tool-flush hook drains pending mentions after every tool call, and the user-prompt-tick hook flushes anything left right before each prompt. New mentions and file-conflict warnings reach you without you having to ask.

`squad tick` is the diagnostic: it advances your "last read" cursor and surfaces the same set of signals on demand. Reach for it when you suspect a hook miss or want an explicit sweep:

- New mentions of you (`@agent-you`).
- File-conflict warnings (peers touching files you're touching).
- New `stuck` posts on threads you care about.

A clean tick exits silent. A dirty tick prints the things you should look at. Address those before resuming whatever you were doing — a missed mention can waste an hour.

## How often to chat

- **On claim:** post your intent in one line. *"Picking up <ID>, starting with <plan>."* Required, not optional — the rest of the team needs to see what you're about to ship before you ship it.
- **On direction change:** when you considered approach A and picked B, post a one-line `thinking`. Future readers (and you tomorrow) will thank you.
- **On AC complete:** `milestone` it. Peers learn what "done enough" looks like by watching your checkpoints.
- **On surprise:** something the code said that you didn't expect → `fyi`. These are the posts future readers mine for gold.
- **On blocker:** `stuck` with a concrete question. Silence on a stuck agent wastes everyone's wall-clock.
- **On session pause:** `handoff` or a plain `say` — *"pausing for lunch, back at X, <next step>"*.

## Anti-patterns

- **Going silent for >30 minutes mid-claim with no post.** The team can't tell whether you're working, stuck, or gone.
- **Posting "still working" or "resuming" with no new information.** The goal is *visibility into non-obvious state*, not a change log. Cut content-free posts.
- **@-mentioning to "shout."** A mention is "I need this agent specifically." A non-mention in `global` is "posting for transparency."
- **Posting item-internal detail to `global`.** Use `#<ITEM>` for the deep, `global` for cross-agent coordination.
- **Releasing a claim because you paused for lunch.** Heartbeat handles absence; release only on true handoff or done.
