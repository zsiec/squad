---
id: FEAT-054
title: I want to be able to click on an agent and see the diff of all the files changed in real time between the work tree and main. I want it to update in real time as changes are added, so I can watch the changes from the browser.
type: feature
priority: P0
area: web
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: web
captured_at: 1777341865
accepted_by: web
accepted_at: 1777342521
references: []
relates-to: []
blocked-by: []
auto_refined_at: 1777342497
auto_refined_by: claude
---

## Problem
The web dashboard surfaces which agents currently hold claims but offers no way to inspect what code each agent has actually changed. Reviewing or babysitting an agent's work requires leaving the browser and running `git diff` against the agent's worktree by hand — there is no live, in-browser view of the agent's pending changes, so a watcher cannot follow progress without context-switching to a terminal.

## Context
Each claim is provisioned with its own per-claim worktree on a dedicated branch (see `internal/claims/` and the worktree provisioning in `cmd/squad/claim.go`). The dashboard is served by `internal/server/`, with the SPA at `internal/server/web/`, and already pushes events to clients over SSE. File-touch tracking lives in `internal/touch/` and can be reused as the change-detection trigger for a given worktree. The natural source of truth for the diff is `git diff main...<worktree-branch>` for the claim, so this feature can layer on top of the existing SSE transport rather than introducing a new channel.

## Acceptance criteria
- [ ] Clicking an agent row in the dashboard SPA opens a diff panel listing every file changed in that agent's worktree relative to `main`, covering added, modified, and deleted files.
- [ ] The diff panel renders unified-diff hunks for each changed file (not just filenames), with per-file expand/collapse.
- [ ] While the panel is open, the SPA refetches the diff every 20 seconds so the watcher sees fresh changes without a page reload. Closing the panel or navigating away cancels the polling interval.
- [ ] A Go test in `internal/server/` exercises the new diff endpoint against a fixture worktree and asserts the response body reflects the worktree-vs-`main` delta (added, modified, deleted files with hunks).

## Design note (2026-04-28)

The original AC called for SSE-driven sub-2s real-time updates with
server-side subscriber tracking. After scope review the user chose
client-side polling at 20s instead — much simpler (no SSE wiring, no
subscriber lifecycle, no server-side change detection), and the
"watching the agent work from the browser" use case tolerates the
20s lag. The dropped AC bullets:

- "within 2 seconds via SSE" → "every 20s via client poll"
- "Go test asserts SSE event emitted" → just the diff endpoint test
- "subscriber count returns to zero" → moot; cancellation is a
  SPA-side `clearInterval` only.
