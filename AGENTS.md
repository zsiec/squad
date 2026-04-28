<!-- do not edit by hand; regenerate with squad scaffold agents-md -->

# AGENTS.md

Generated from current ledger state. CLAUDE.md is the only hand-edited contract file.

## Ready

- **FEAT-064** (P1) — spa range-selection ui for comment-driven auto-refine
- **FEAT-060** (P3) — area ownership auto-mentions top closer when items file or scope splits
- **FEAT-061** (P3) — auto-postmortem fires when a claim ends without durable learning artifacts

## In flight

- **FEAT-067** — (orphan — item file missing) · @agent-bbf6 · squad go auto-claim
- **CHORE-020** — (orphan — item file missing) · @agent-afcd · squad go auto-claim
- **FEAT-066** — (orphan — item file missing) · @agent-310b · full breaker: --reason enum + remove squad blocked + migration + SPA + docs in 5 sequential phases
- **FEAT-068** — (orphan — item file missing) · @agent-401f · squad go auto-claim

## Recently done

- **CHORE-019** — stats panel renderInsights races on rapid window flips — last-fetch-wins shows stale data — render-token sentinel: stale renderInsights calls bail post-await; latest window wins; no chart accumulation
- **FEAT-063** — If there is an idle agent, there should be a way in the SPA for me to click and drag an item to that agent and have it start working on it. — operators can drag ready items onto idle agents in the SPA; new POST /api/items/{id}/assign creates the claim with audit-trail intent
- **FEAT-062** — A captured item should allow operators to comment inline on selected ranges and submit with comments, with context for regeneration (using claude cli lke auto-refine) — auto-refine endpoint accepts comments and broadens status gate; spa ui + peer-queue cleanup deferred to follow-ups
- **FEAT-059** — squad retro generates weekly digest of failure modes and process suggestions — weekly retro digest: failure modes + slowest type + rule-based recommendation; auto-loads via skill paths
- **FEAT-058** — risk-tiered code review requires two reviewers for p0 and high risk items — tier-aware code-review gate: P0/risk:high require two distinct reviewers; --force still bypasses with audit trail
- **FEAT-057** — standup digest at squad go shows active peer claims — squad go now prints a peers: block before the AC line — agent display name, item id, area, age (last_touch), and latest human-verb chat excerpt. Sort by last_touch DESC, cap 6 rows + '+N more'. When the new claim's area overlaps with a peer's, a single-line nudge precedes the digest suggesting 'squad ask @<peer> ...'. Excerpts cap at peerDigestCap before the per-row query so a busy repo doesn't pay N+1. Resume path prints the digest too (no overlap nudge — choice is already made). Four unit tests cover: zero peers, one peer no overlap, one peer with overlap, seven peers truncated to 6 + 1 more. Reviewer flagged three CONCERNs (N+1, resume-path skip, dead resort comment); all three folded into this commit.
- **FEAT-056** — can you redesign the stats page including more stats and better visualization? — 8 tiles backed by existing stats.Snapshot (3 preserved + 5 new: item-mix doughnut, claim p50/p90/p99 bars, verify-by-kind stacked, agent leaderboard top 10, by-capability horizontal); window selector switches 24h/7d/30d in place; emptyCanvas helper unifies empty-state across canvases; no backend changes; structural Go test pins all tile ids + selector + window options + 'no data' copy
- **FEAT-055** — diff panel needs prettifying — color +/- lines, syntax highlighting, wider layout, clearer file separators — SPA-only prettify pass on the FEAT-054 diff panel. Per-line <div> blocks classified by leading character (addition/deletion/hunk-header/file-header/context); CSS color-codes them in standard diff palette. Each file is now a bordered card with a chevron-toggle header; busy diffs read as a stack instead of a wall. Drawer widens to min(70vw, 1100px) when files are present via a data-mode attribute; timeline-only mode is unchanged. No backend or test changes; FEAT-054 tests still pass.
- **BUG-054** — auto-refine spa toast does not surface the new stdout field — auto-refine toast now reads both stdout and stderr; prefers stdout since claude -p emits diagnostics there
- **FEAT-054** — I want to be able to click on an agent and see the diff of all the files changed in real time between the work tree and main. I want it to update in real time as changes are added, so I can watch the changes from the browser. — GET /api/agents/{id}/diff returns worktree-vs-merge-target diff (committed branch divergence + uncommitted edits + untracked files) per the agent's most recent active claim. SPA agent-detail drawer renders the diff in a new collapsible section beneath the timeline; setInterval(20s) refresh while open, clearInterval on close. AC was rewritten before implementation to drop the SSE/subscriber-tracking shape in favor of client-side polling per user direction; design note recorded in the item body. Live-verified against the running launchd daemon: own-agent diff returned 9 files matching reality.

## Specs

- [Per-agent activity stream — Claude Code tool-call events surfaced in the SPA](.squad/specs/agent-activity-stream.md)
- [Agent-team management surface](.squad/specs/agent-team-management-surface.md)
- [Auto-refine inbox items via the local claude CLI](.squad/specs/auto-refine-inbox.md)
- [Drive agent uptake of attestations, learnings, peer chat, and milestones](.squad/specs/feature-uptake-nudges.md)
- [Inbox refinement loop — captured items can be sent back for refinement with reviewer comments](.squad/specs/inbox-refinement.md)
- [Interview-driven intake](.squad/specs/intake-interview.md)
- [MCP-driven dashboard bootstrap and upgrade hygiene](.squad/specs/mcp-dashboard-bootstrap.md)

## Epics

- [agent-activity-stream](.squad/epics/agent-activity-stream.md) — active
- [auto-refine-inbox](.squad/epics/auto-refine-inbox.md) — active
- [cadence-and-time-boxing-as-pacing](.squad/epics/cadence-and-time-boxing-as-pacing.md) — active
- [capability-routing](.squad/epics/capability-routing.md) — active
- [coordination-defaults-opinionated-opt-out](.squad/epics/coordination-defaults-opinionated-opt-out.md) — active
- [documentation-contract-generated-agents-md](.squad/epics/documentation-contract-generated-agents-md.md) — active
- [feature-uptake-nudges](.squad/epics/feature-uptake-nudges.md) — active
- [first-run-dashboard](.squad/epics/first-run-dashboard.md) — active
- [inbox-refinement](.squad/epics/inbox-refinement.md) — done
- [intake-interview-cli](.squad/epics/intake-interview-cli.md) — active
- [intake-interview-core](.squad/epics/intake-interview-core.md) — active
- [intake-interview-mcp](.squad/epics/intake-interview-mcp.md) — active
- [intake-interview-plugin](.squad/epics/intake-interview-plugin.md) — active
- [observation-to-knowledge-pipeline](.squad/epics/observation-to-knowledge-pipeline.md) — active
- [refinement-and-contract-hardening](.squad/epics/refinement-and-contract-hardening.md) — active
- [team-practices-as-mechanical-visibility](.squad/epics/team-practices-as-mechanical-visibility.md) — active

