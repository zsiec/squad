---
title: Agent-team management surface
motivation: "Human SWE practices (PR review, sprint planning, ADRs, postmortems, code ownership) evolved around constraints AI agents do not share — and around capabilities agents lack. Mapping them 1:1 misses the load-bearing differences: agents have no persistent memory between sessions; coordination is mechanically cheap, not socially expensive; structured I/O outperforms prose; agents do not hesitate to ask but also do not intuit when to ask; capability is per-claim, not talent-based; action bias dominates over reflection; long-running context decays via compaction. This spec captures the asymmetries and codifies the principles squad should optimize for: opinionated default-on coordination, a mechanical observation-to-knowledge pipeline, and structured contracts that are enforced rather than recommended. Audit of the repo's own dogfooding (97 done items at the time of writing) shows the right primitives exist (worktrees, touches, learning artifacts, specs/epics, capability tags would-be) but adoption is below the level the workflow needs them. Touches are not populated; --worktree is never used despite the migration just landing protection for the column it would write; learning artifacts contain a single proposed gotcha that is itself a meta-observation about the learning capture friction; refinement history shows on 1% of done items."
acceptance:
  - Coordination primitives (worktrees, touches) are on by default in scaffolded repos rather than opt-in.
  - Observation events (handoff surprised-by bullets, doctor-detected drift, code-reviewer rejected findings) flow into the learning ledger automatically, not by an agent remembering to file.
  - Item acceptance criteria are enforced as testable propositions before claim — placeholder, vague, or stub AC blocks claim with a clear message.
  - Items above a threshold of acceptance bullets that map to distinct files are decomposed before claim.
  - The 2-hour exploration cap is enforced via async wakeup, not just documented in a skill.
  - Capability tags on items route the ready stack so agents only see work they are equipped for.
  - AGENTS.md is generated from current ledger state; CLAUDE.md is the only hand-edited contract file.
non_goals:
  - Replicating Jira / sprint mechanics — squad is a workflow framework, not a project tracker.
  - Modeling humans-in-the-loop social dynamics like psychological safety, retro discussions, or tribal knowledge accumulation.
  - Adding new chat channels beyond the existing #global thread.
  - Multi-tenant or organization-level features — single-repo dogfooding is the audience.
integration:
  - "internal/store: schema additions for capability tags and item-level dependency hints; bootstrap markers stay locked."
  - "internal/intake: refinement contract enforcement; commit writer must render bundle bodies into item markdown (currently broken — see Epic B)."
  - "internal/hygiene: doctor drift detection emits learning_propose with diagnostic context."
  - "internal/learning: auto-ingest pipeline from handoff, doctor, and code-reviewer outputs."
  - "plugin/hooks: pre_edit_touch_check.sh activated and populates .squad/touches/."
  - "plugin/skills: squad-handoff, squad-loop, squad-cadence updated to enforce the new defaults."
  - "cmd/squad: squad init scaffolds opinionated config; squad next filters by capability; squad scaffold renders AGENTS.md."
risks: []
open_questions: []
intake_session: intake-20260427-44256e4424c4
---

## Background
(Filled in during intake.)
