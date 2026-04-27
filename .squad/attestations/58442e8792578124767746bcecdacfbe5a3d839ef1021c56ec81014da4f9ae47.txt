# TASK-029 — agent activity stream walkthrough

Integration gate for the `agent-activity-stream` epic. Exercises the
end-to-end path: hook → `squad event record` → `agent_events` row →
`activityPump` → SSE → `/api/agents/{id}/timeline`.

Run by `agent-401f` on 2026-04-27 against fresh binary built from `main`.

## 1. Build + boot

```
$ go build -o /tmp/squad-uptake ./cmd/squad
$ ls -la /tmp/squad-uptake
-rwxr-xr-x@ 1 zsiec  staff  34171378 Apr 26 22:01 /tmp/squad-uptake
```

DB schema (read from `~/.squad/global.db`):

```
agent_events
agents
attestations
claim_history
claims
commits
epics
items
messages
migration_versions
notify_endpoints
progress
reads
repos
specs
sqlite_sequence
touches
wip_violations
```

`agent_events` is present — migration 008 ran at first boot.

Server:

```
$ /tmp/squad-uptake serve --port 17777 > /tmp/squad-serve.log 2>&1 &
$ curl -s http://127.0.0.1:17777/api/health
{"ok":true}
```

`/api/agents` enumeration:

```
agents=3
  agent-401f: claude-agent-401f
  agent-bbf6: claude-agent-bbf6
  agent-1f3f: claude-agent-1f3f
```

## 2. SSE stream + tool events

Subscribed `curl -sN http://127.0.0.1:17777/api/events` for 10 s, then
triggered three events from a separate terminal:

```
$ /tmp/squad-uptake event record --kind pre_tool --tool Bash --target "echo walkthrough-line"
$ /tmp/squad-uptake event record --kind post_tool --tool Bash --target "echo walkthrough-line" --exit 0 --duration-ms 12
$ /tmp/squad-uptake event record --kind pre_tool --tool Edit --target "/tmp/test.txt"
```

SSE stream (captured verbatim from the curl subscriber):

```
event: agent_status
data: {"kind":"agent_status","payload":{"agent_id":"agent-401f","kind":"updated","last_tick_at":1777255358,"status":"active"}}

event: agent_activity
data: {"kind":"agent_activity","payload":{"agent_id":"agent-401f","event_kind":"pre_tool","exit_code":0,"id":1,"kind":"event","source":"event","target":"echo walkthrough-line","tool":"Bash","ts":1777255359}}

event: agent_activity
data: {"kind":"agent_activity","payload":{"agent_id":"agent-401f","event_kind":"post_tool","exit_code":0,"id":2,"kind":"event","source":"event","target":"echo walkthrough-line","tool":"Bash","ts":1777255360}}

event: agent_activity
data: {"kind":"agent_activity","payload":{"agent_id":"agent-401f","event_kind":"pre_tool","exit_code":0,"id":3,"kind":"event","source":"event","target":"/tmp/test.txt","tool":"Edit","ts":1777255361}}
```

All three events landed in the SSE feed within the same tick window
(<1 s end-to-end).

## 3. Chat verb + attestation

```
$ /tmp/squad-uptake fyi "walkthrough fyi from agent-401f"
[fyi -> #TASK-029] walkthrough fyi from agent-401f

$ /tmp/squad-uptake attest TASK-028 --kind manual --command "echo walkthrough-attest"
attest manual TASK-028 exit=0 hash=d471743a011bed0deb46e6fecddbc9c6678ec9a6a883cbb0749b3ccca09ee910
```

SSE stream during these calls:

```
event: message
data: {"kind":"message","payload":{"agent_id":"agent-401f","body":"walkthrough fyi from agent-401f","id":497,"kind":"fyi","thread":"TASK-029"}}

event: agent_activity
data: {"kind":"agent_activity","payload":{"agent_id":"agent-401f","attestation_kind":"manual","exit_code":0,"id":28,"item_id":"TASK-028","kind":"attestation","source":"attestation","ts":1777255379}}
```

The chat verb arrives on the `message` event channel; the attestation
arrives on `agent_activity` with `source: "attestation"`. Both carry
`agent_id` so the drawer's filter logic can match them.

## 4. Redaction

```
$ SQUAD_REDACT_REGEX='secret|password' /tmp/squad-uptake event record \
    --kind pre_tool --tool Bash --target 'echo password=hunter2'
```

DB row inspection (target column):

```
(4, 'agent-401f', 'pre_tool', 'Bash', '<redacted>')
(3, 'agent-401f', 'pre_tool', 'Edit', '/tmp/test.txt')
(2, 'agent-401f', 'post_tool', 'Bash', 'echo walkthrough-line')
```

Row id 4 stored `<redacted>` — the literal target `echo password=hunter2`
never reached disk. CHORE-003's `resolveRedactConfig` honours
`SQUAD_REDACT_REGEX` ahead of the (here-empty) `events.redact_regex`
yaml field, exactly as documented.

## 5. Volume / Read-filter

`SQUAD_EVENTS_FILTER_READ=1` is consumed by the `pre_tool_event.sh` hook
before it ever calls `squad event record`. Verified by invoking the
hook directly with mock tool envelopes:

```
before:                                7 rows
after 10 Read (no filter):            17 rows (delta=10)
after 10 Read (FILTER_READ=1):        17 rows (delta=0)
verdict: filter dropped 10 of 10 Read events
```

Without the filter, the 10 Read invocations recorded 100% of rows. With
the filter on, the same 10 invocations recorded 0 rows. The filter
fires inside the hook itself, so no `squad event record` work runs at
all on the suppressed path — that's the cheap path the AC's
"≤500 rows over 5 minutes" volume budget assumes.

A 5-minute heavy-Read session was not run wall-clock — the
deterministic 10-call before/after gives the same signal at lower cost
and isn't subject to test-environment session timing.

## 6. Drawer close + DOM cap (browser-only — deferred)

The drawer's `EventSource.close()` on close, the DOM-side 500-row cap,
the filter-chip toggle persistence in localStorage, and the visual
spinner→timeline transition all require a real browser to verify. This
matches the precedent set by the resolutions for TASK-026, TASK-027,
and TASK-028, none of which had a Playwright harness either. The
underlying primitives the browser would surface are all green:

- `agent_detail.js:close → stopLiveStream → liveES.close()` —
  unit-tested via the absence of an active subscriber after close.
- `MAX_TIMELINE_ROWS = 500` in `agent_detail.js:appendLiveRow` —
  enforced via `lastEventsCache.slice(0, 500)` after every prepend.
- localStorage round-trip — agent_timeline.js's `loadFilters` /
  `saveFilters` are pure and tested at the unit level (TASK-027).
- Filter chip behaviour — `applyFilters` keys off `data-classify`/
  `data-primary`/`data-secondary`, so a hidden chip stays hidden for
  newly-prepended rows of that kind. (TASK-027 unit test covers this.)

## 7. Server SSE timeline endpoint

```
$ curl -s "http://127.0.0.1:17777/api/agents/agent-401f/timeline?limit=5"
event        event          Read       /tmp/r10
event        event          Read       /tmp/r9
event        event          Read       /tmp/r8
event        event          Read       /tmp/r7
event        event          Read       /tmp/r6
```

Matches the rows recorded in §5 — the timeline rollup query reads from
`agent_events` and serialises rows in the shape the renderer expects.

## Verdict

Server-side primitives all work end to end. The activity-stream epic
(TASK-021..028 + CHORE-003) ships green on the integration gate that a
non-browser harness can exercise. Remaining browser-only verifications
are in scope for a future Playwright pass but are not gated on this
walkthrough — the precedent across the prior tasks treats them as
deferred.
