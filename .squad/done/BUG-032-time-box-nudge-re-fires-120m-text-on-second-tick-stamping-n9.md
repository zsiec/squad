---
id: BUG-032
title: time-box nudge re-fires 120m text on second tick stamping n90 without 90m print
type: bug
priority: P2
area: cmd/squad
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777323350
accepted_by: web
accepted_at: 1777324024
references: []
relates-to: [FEAT-042]
blocked-by: []
---

## Problem

`maybePrintTimeBoxNudge` in `cmd/squad/cadence_nudge.go` reprints the 120m
"hard cap" nudge on every tick after the first 120m fire whenever
`nudged_90m_at` was never stamped (either because the 90m nudge was
silenced by a recent milestone, or because the claim ticked through 90m →
120m without an intervening Bash boundary). On the second tick the 90m
branch enters (`!nudged90m.Valid` is true), calls
`timeBoxNudgeText(claimAge, …)` where `claimAge >= 120m`, which returns
the **120m** text — not the 90m text — and stamps `nudged_90m_at`. Net
effect: the 120m line prints twice, and the 90m thinking-prompt is
permanently swallowed for that claim.

## Context

The 120m branch (line 161) and 90m branch (line 171) each gate on their
own marker column being NULL, but `timeBoxNudgeText` (line 115) uses the
claim age alone to choose copy: if `claimAge >= 120m` it returns the 120m
text regardless of which branch called it.

`TestMaybePrintTimeBoxNudge_120mSkipsUnfired90m` (cadence_nudge_test.go:245)
verifies the FIRST tick at 121m fires the 120m nudge and leaves
`nudged_90m_at` NULL — but it stops there. The bug surfaces on the
SECOND tick.

The bug is also reachable through the silenced-90m path: a claim crossing
90m with a recent milestone correctly suppresses the 90m print and leaves
`nudged_90m_at` NULL by design (so it can fire later). When the claim
subsequently crosses 120m, the 120m nudge fires, and any further tick
re-emits the 120m text under an n90 stamp.

This was self-disclosed in spirit by the FEAT-042 resolution note about
"marker-on-fire (not on-cross)" as a feature, but the precedence in
`timeBoxNudgeText` was not adjusted to match.

## Acceptance criteria

- [ ] On the second tick after a successful 120m fire, `maybePrintTimeBoxNudge` produces no output (or a 90m text that is suppressed by the 120m branch having already fired).
- [ ] The reproducer test (see Notes) goes from FAIL to PASS without weakening any existing assertion.
- [ ] The 90m branch never stamps `nudged_90m_at` while emitting the 120m hard-cap copy. Either it emits real 90m copy, or it stays silent.
- [ ] New regression test in `cmd/squad/cadence_nudge_test.go` covers the silenced-90m → crosses-120m → second-tick path and asserts no double-fire.

## Notes

The simplest fix is to gate the 90m branch on `claimAge < timeBoxThreshold120m`. A claim past the hard cap should not re-enter the 90m branch even when its marker is NULL — once 120m has been served, the 90m slot is moot.

Alternatively, change `timeBoxNudgeText` so the 90m branch passes a flag and only the 120m branch path can return the 120m string. Same effect, more invasive.

Verified RED test (this exact body fails on `main`):

```go
func TestRepro_120mDoubleFireAfter90mSkipped(t *testing.T) {
    t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
    env := newTestEnv(t)
    now := time.Now()
    claimAt := now.Add(-121 * time.Minute)
    if _, err := env.DB.Exec(
        `INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, ?, ?, '', 0)`,
        env.RepoID, "BUG-9001", env.AgentID, claimAt.Unix(), claimAt.Unix(),
    ); err != nil { t.Fatalf("seed claim: %v", err) }

    var buf1 bytes.Buffer
    maybePrintTimeBoxNudge(context.Background(), env.DB, env.RepoID, env.AgentID, now, &buf1)
    if !strings.Contains(strings.ToLower(buf1.String()), "handoff") {
        t.Fatalf("first tick should fire 120m nudge; got %q", buf1.String())
    }

    var buf2 bytes.Buffer
    maybePrintTimeBoxNudge(context.Background(), env.DB, env.RepoID, env.AgentID, now.Add(time.Minute), &buf2)
    if strings.Contains(strings.ToLower(buf2.String()), "handoff") {
        t.Errorf("BUG: 120m nudge re-fired on second tick; got %q", buf2.String())
    }
    var n90, n120 sql.NullInt64
    _ = env.DB.QueryRow(`SELECT nudged_90m_at, nudged_120m_at FROM claims WHERE item_id=?`, "BUG-9001").Scan(&n90, &n120)
    if n90.Valid {
        t.Errorf("BUG: nudged_90m_at stamped without ever printing 90m text; n90=%v n120=%v", n90, n120)
    }
}
```

Observed failure on `main` (verified during BUG-032 capture):

```
feat042_repro_test.go:41: BUG: 120m nudge re-fired on second tick; got "  tip: 2h time-box hit — `squad handoff` ..."
feat042_repro_test.go:49: BUG: nudged_90m_at stamped without ever printing the 90m text; n90={1777323400 true} n120={1777323340 true}
```

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
