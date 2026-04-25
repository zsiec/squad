---
description: Resume the operating loop — register with the team, tick, and surface the top of the ready stack so you can pick the next item.
args: ""
---

You are starting (or resuming) work on a squad-managed repo. Run the loop entry steps:

1. Register and tick in one bash invocation:

```bash
squad register --as "agent-$(openssl rand -hex 2)" --name "agent-XXXX" && squad tick
```

2. Show the top of the ready stack:

```bash
squad next
```

Then, before doing anything else:

- If you have an existing in-progress claim, continue it (announce in one line).
- Otherwise, pick the top item from `squad next`, verify its `blocked-by:` is clear, and announce the pick: *"Picking up <ID>: <title>."*
- If the picked item is estimated >1h, write a brief plan in `docs/plans/` (or your repo's plans dir) before starting.
- If there are unread mentions from `squad tick`, address them BEFORE continuing.

Then invoke the `squad-loop` skill and proceed step 3 onward (claim → work → checkpoint → test → review → commit → done).
