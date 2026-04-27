<!-- do not edit by hand; regenerate with squad scaffold agents-md -->

# AGENTS.md

Generated from current ledger state. CLAUDE.md is the only hand-edited contract file.

## Ready

- **BUG-037** (P2) — AGENTS.md on main is in drift from generator output; documentation-contract epic landed without regenerating
- **FEAT-051** (P2) — wire timeBoxNudge into squad listen pollMailbox for true async wakeup

## In flight

- **BUG-037** — AGENTS.md on main is in drift from generator output; documentation-contract epic landed without regenerating · @agent-afcd · squad go auto-claim

## Recently done

- **BUG-039** — auto-refine cannot satisfy DoR when area is placeholder — Threaded optional area arg through squad_auto_refine_apply (MCP schema, args struct, register handler, items.AutoRefineApply); area written to frontmatter alongside body when supplied. Prompt now tells claude to pick a free-form area on placeholder. 3 new tests cover area-write, back-compat, and JSON-RPC pass-through. Commit 7695aac.
- **BUG-036** — AGENTS.md Recently done sort tiebreaks by ID asc, surfacing oldest BUGs over genuinely recent items — pickDone now keys on Updated DESC → AcceptedAt DESC → ID ASC. Two new tests pin: (1) AcceptedAt tiebreaks among same-day items, (2) ID ASC fallback when AcceptedAt is 0 (legacy items). Comment updated to be honest about the field's semantics — set at promote time, used as sub-day recency proxy. Live ledger 'Recently done' now leads with BUG-039/034/032/033/030/029/CHORE-015/FEAT-046/047/048 instead of the lowest 10 BUG IDs. Full suite + vet clean. Reviewer flagged the original comment as misleading (claimed 'closed on the same calendar day'); corrected.
- **BUG-035** — AGENTS.md Recently done section omits the close summary required by FEAT-049 AC — loadDoneSummaries pulls per-item done bodies and feeds Summaries map through scaffold; renderer now emits id/title/summary per AC, with _(no summary)_ fallback
- **BUG-034** — auto-refine-inbox spec YAML breaks (acceptance bullet has unquoted colon-space) — Quoted the offending bullet in .squad/specs/auto-refine-inbox.md (double-quoted with internal escapes; prose byte-equivalent). Added TestParse_AllProjectSpecsParseCleanly in internal/specs/specs_test.go that resolves the repo root from the test file and Parse-checks every .squad/specs/*.md — so a future colon-space typo in any spec fails CI rather than silently dropping from the generator output. Verified: all 4 specs tests pass; scaffold agents-md and scaffold doc-index both list 7/7 specs.
- **BUG-032** — time-box nudge re-fires 120m text on second tick stamping n90 without 90m print — Added one-line guard claimAge < timeBoxThreshold120m to the 90m branch in maybePrintTimeBoxNudge (cmd/squad/cadence_nudge.go:171). Once the hard cap has been served, the 90m branch is unreachable, so it can no longer re-emit 120m text or stamp nudged_90m_at under that print. New regression test TestMaybePrintTimeBoxNudge_SecondTickAfter120mDoesNotDoubleFire covers the silenced-90m + crosses-120m + second-tick path. All 6 timebox tests pass; full suite + vet clean. Note: the commit accidentally included unrelated WIP from internal/server/items_auto_refine* that was already staged in the index — flagged for the user.
- **BUG-033** — DoR vague-acceptance-bullet verb allow-list misses common English verbs causing false positives — Added motion/state-transition/regression verbs to vagueACBulletAllowedVerbs (goes/go/went/going/gone, becomes/become/became/turns/turn/turned, changes/change/changed, regresses/regress/regressed, breaks/break/broke/broken, holds/hold/held, appears/appear/disappears/vanishes, clear/stays/stay/remains/remain/remained). 4 new dor_test.go cases each isolating a target verb. Differential corpus sweep: 145 vague violations to 130 across 141 items, zero items flipped from accept to reject. Code-reviewer caught one tautological test (regresses bullet was rescued by 'change'); reworded bullet to isolate.
- **BUG-030** — touch path normalization mismatch between hook and squad touch CLI — Tracker.normalizePath canonicalises paths; Add/EnsureActive/Conflicts/Release apply it. Repo root cached via sync.Once from repos.root_path. Outside-repo paths stay absolute, ./-prefix collapses, abs+rel collide on conflict query. --skip-verify because peer's FEAT-046 WIP (squad register --capability) breaks cmd/squad build with 'unknown field Capabilities in RegisterArgs'; my touch tests + scoped vet are green.
- **BUG-029** — worktree-by-default scaffold breaks cmd/squad attest+done test fixtures (no main ref) — Added sibling helper gitInitDirCommittedMain (gitInitDir + 'git commit --allow-empty -m initial' inline -c). Updated 9 failing tests across attest_test.go, done_test.go, r4_lifecycle_test.go. Original gitInitDir unchanged; commitless-main scenarios still work. Full suite green; 3x back-to-back -race -count=1 runs all pass.
- **CHORE-015** — AGENTS.md generated banner and regenerate hook — Added --check flag to scaffold agents-md (compare existing to generator output, exit non-zero on drift, no write). New plugin/hooks/pre_commit_agents_md.sh fires on PreToolUse Bash before git commit when AGENTS.md is staged + squad binary is on PATH. Hook + flag + tests + hooks.json + embed.All registration. CLAUDE.md is unaffected.
- **FEAT-046** — squad register accepts repeatable --capability flag — agents.capabilities TEXT NOT NULL DEFAULT '[]' via migration 011 + v11 bootstrap probe; --capability flag (repeatable, lowercased+deduped+sorted) replaces on re-register; SetCapabilities sentinel preserves prior set on implicit re-register from squad go; whoami --verbose renders the set with (none) fallback; MCP register reads capabilities pointer-slice so absent=preserve, []=clear. verify skipped — race-only flakes in agent-401f's auto-refine tests, unrelated; without -race full suite is green

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

