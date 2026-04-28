---
id: BUG-047
title: auto-refine spawns claude with daemon's cwd so squad mcp subprocess cannot discover the repo and squad_get_item returns not found
type: bug
priority: P2
area: internal/server
status: open
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-bbf6
captured_at: 1777335757
accepted_by: agent-bbf6
accepted_at: 1777335795
references: []
relates-to: [BUG-044, BUG-040]
blocked-by: []
---

## Problem

`autoRefineCommand` in `internal/server/items_auto_refine.go` spawns
`claude -p` without setting `cmd.Dir`, so the subprocess inherits the
daemon's cwd. Under launchd / systemd-user that is `/`. The spawned
claude then launches `squad mcp` (also in `/`), which calls
`repo.Discover(wd)`, walks up from `/`, and fails. The MCP server has
no repo context, so `squad_get_item("BUG-XXX")` returns "not found"
no matter what argument claude passes. Claude reads the failure,
decides there is nothing to refine, and exits cleanly without calling
`squad_auto_refine_apply`. The handler then sees
`after.AutoRefinedAt <= before.AutoRefinedAt` and surfaces the
generic "claude exited without drafting; run again" 500.

BUG-044 fixed the *handler's* preflight — `items.FindByID(squadDir,
id)` now uses the resolved squadDir from `resolveItemRepo`. But the
*subprocess* still spawns with the wrong cwd, so the user-facing
"run again" error persists. Reproduced today.

## Context

`internal/server/items_auto_refine.go:73` builds the command:

```go
func autoRefineCommand(ctx context.Context, prompt, mcpConfigPath string) *exec.Cmd {
    cmd := exec.CommandContext(ctx, "claude", "-p", prompt,
        "--mcp-config", mcpConfigPath,
        "--strict-mcp-config",
        "--allowedTools", autoRefineAllowedToolsArg(),
    )
    cmd.Env = append(os.Environ(), "SQUAD_NO_HOOKS=1")
    autoRefineSetProcessGroup(cmd)
    return cmd
}
```

No `cmd.Dir` set. The companion handler resolves the item's repo via
`resolveItemRepo` → returns `(repoID, squadDir)` where `squadDir =
<root_path>/.squad`. The repo root is `filepath.Dir(squadDir)`.

A second concern surfaced during diagnosis: the handler uses
`cmd.Output()` which discards stdout. When the subprocess exits 0
without drafting, the operator gets the generic "run again" with no
hint as to why. Capturing stdout and including a truncated tail in
the 500 response body would let the operator see what claude
actually said (e.g. "I couldn't find item BUG-044") instead of
guessing.

## Acceptance criteria

- [ ] `autoRefineCommand` accepts the resolved repo root (or the squadDir, with the helper deriving the parent) and sets `cmd.Dir = repoRoot` on the spawned subprocess.
- [ ] After this change, an auto-refine run against an item in workspace mode produces a successful draft — the spawned claude calls `squad_auto_refine_apply` and `auto_refined_at` advances.
- [ ] When the subprocess exits 0 without advancing `auto_refined_at`, the handler's 500 response includes a truncated tail of the subprocess's stdout so the operator can see what claude said. Existing single-repo behavior preserved.
- [ ] New regression test in `internal/server/items_auto_refine_test.go` pins that the spawned `*exec.Cmd`'s `Dir` field equals the resolved repo's root_path. Existing tests continue to pass.

## Notes

The simplest plumbing: change `autoRefineCommand`'s signature to take
`repoRoot string` and assign `cmd.Dir = repoRoot` after the
`exec.CommandContext` call. The handler already has the squadDir
from `resolveItemRepo`; one `filepath.Dir` call yields the root.

Alternatively, extend `resolveItemRepo` to return the root_path
explicitly. Cleaner long-term — multiple callers will eventually
need it — but a single-line `filepath.Dir(squadDir)` at the call
site is enough for V1.

For the stdout-capture concern: change `cmd.Output()` to
`cmd.CombinedOutput()` (or pipe stdout into a separate buffer) and
thread the captured bytes through `autoRefineRunResult`. The 500
body already has a `stderr` field; mirror it with `stdout`.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
