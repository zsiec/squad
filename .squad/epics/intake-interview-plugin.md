---
spec: intake-interview
status: open
parallelism: |
  Single-task epic. Depends on the MCP and CLI epics being complete and the
  manifest schema being available.
---

## Goal

Ship the user-facing surface in the Claude Code plugin: a `/squad:squad-intake`
slash command, a skill at `plugin/squad/skills/squad-intake/SKILL.md` that
drives the interview, and a manifest update that registers the four new MCP
tools and bumps the plugin version.

## Scope

- `plugin/squad/commands/squad-intake.md` — thin slash command, matches the
  format of `plugin/squad/commands/squad-work.md`.
- `plugin/squad/skills/squad-intake/SKILL.md` — operating manual for the
  interview agent: one question at a time, multiple-choice when possible,
  the literal "Ship it? (y/n/edit)" sign-off prompt, hard rules against
  inventing acceptance criteria or silently passing --ready.
- `plugin/manifest.json` — register the four MCP tools, semver-minor bump.
- Manual smoke test in a fresh tmp repo (green-field + refine).

## Out of scope

- Automated test coverage for plugin markdown (no harness exists).
- Refactoring of the existing skills.
