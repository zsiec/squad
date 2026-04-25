// Package hygiene runs integrity sweeps over the squad store and the
// on-disk items directory: stale claims, ghost agents, orphan touches,
// broken file references, items in done/ still marked in_progress, and
// SQLite-integrity failures.
package hygiene

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"time"
)

const (
	StaleClaimSec     = 30 * 60
	StaleClaimLongSec = 2 * 60 * 60
	GhostAgentSec     = 24 * 60 * 60
)

type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarn
	SeverityError
)

type Finding struct {
	Severity Severity
	Code     string
	Message  string
	Fix      string
}

type ItemRef struct {
	ID, Path, Status string
	References       []string
}

type Items interface {
	List(ctx context.Context) ([]ItemRef, error)
}

type Sweeper struct {
	db     *sql.DB
	repoID string
	items  Items
	now    func() time.Time
}

func New(db *sql.DB, repoID string, items Items) *Sweeper {
	return &Sweeper{db: db, repoID: repoID, items: items, now: time.Now}
}

func NewWithClock(db *sql.DB, repoID string, items Items, clock func() time.Time) *Sweeper {
	return &Sweeper{db: db, repoID: repoID, items: items, now: clock}
}

func (sw *Sweeper) nowUnix() int64 { return sw.now().Unix() }

func (sw *Sweeper) Sweep(ctx context.Context) ([]Finding, error) {
	var findings []Finding
	now := sw.nowUnix()

	rows, err := sw.db.QueryContext(ctx, `
		SELECT item_id, agent_id, last_touch, long FROM claims
		WHERE repo_id = ? AND ((long = 0 AND last_touch < ?) OR (long = 1 AND last_touch < ?))
	`, sw.repoID, now-StaleClaimSec, now-StaleClaimLongSec)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item, agent string
		var lt, long int64
		if err := rows.Scan(&item, &agent, &lt, &long); err != nil {
			rows.Close()
			return nil, err
		}
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Code:     "stale_claim",
			Message:  "stale claim: " + item + " (agent=" + agent + ")",
			Fix:      "squad force-release " + item + " --reason \"stale\"",
		})
	}
	rows.Close()

	ghostRows, err := sw.db.QueryContext(ctx,
		`SELECT id, last_tick_at FROM agents WHERE repo_id = ? AND status='active' AND last_tick_at < ?`,
		sw.repoID, now-GhostAgentSec)
	if err != nil {
		return nil, err
	}
	for ghostRows.Next() {
		var id string
		var ts int64
		if err := ghostRows.Scan(&id, &ts); err != nil {
			ghostRows.Close()
			return nil, err
		}
		findings = append(findings, Finding{
			Severity: SeverityWarn,
			Code:     "ghost_agent",
			Message:  "ghost agent: " + id + " (no tick in over 24h, still marked active)",
			Fix:      "squad force-release --agent " + id,
		})
	}
	ghostRows.Close()

	orphRows, err := sw.db.QueryContext(ctx, `
		SELECT t.agent_id, t.path, t.item_id FROM touches t
		LEFT JOIN claims c ON c.item_id = t.item_id AND c.agent_id = t.agent_id AND c.repo_id = t.repo_id
		WHERE t.repo_id = ? AND t.released_at IS NULL AND t.item_id IS NOT NULL AND c.item_id IS NULL
	`, sw.repoID)
	if err != nil {
		return nil, err
	}
	for orphRows.Next() {
		var a, p, i string
		if err := orphRows.Scan(&a, &p, &i); err != nil {
			orphRows.Close()
			return nil, err
		}
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Code:     "orphan_touch",
			Message:  "orphan touch: " + p + " by " + a + " (item " + i + " not claimed)",
			Fix:      "squad untouch " + p,
		})
	}
	orphRows.Close()

	var ic string
	if err := sw.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&ic); err != nil {
		findings = append(findings, Finding{
			Severity: SeverityError, Code: "integrity_check",
			Message: "integrity_check failed: " + err.Error(),
			Fix:     "squad backup; restore from last good snapshot",
		})
	} else if ic != "ok" {
		findings = append(findings, Finding{
			Severity: SeverityError, Code: "integrity_check",
			Message: "integrity_check: " + ic,
			Fix:     "squad backup; sqlite3 ~/.squad/global.db .recover",
		})
	}

	if sw.items != nil {
		refs, err := sw.items.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range refs {
			if r.Status == "in_progress" && strings.Contains(r.Path, "/done/") {
				findings = append(findings, Finding{
					Severity: SeverityWarn, Code: "done_in_progress",
					Message: "item " + r.ID + " is in done/ but frontmatter status is in_progress",
					Fix:     "edit " + r.Path + " — set status: done",
				})
			}
			for _, ref := range r.References {
				if _, err := os.Stat(stripLineSuffix(ref)); err != nil {
					findings = append(findings, Finding{
						Severity: SeverityInfo, Code: "broken_ref",
						Message: "item " + r.ID + " references missing file " + ref,
						Fix:     "edit " + r.Path + " — fix or remove the reference",
					})
				}
			}
		}
	}

	return findings, nil
}

func stripLineSuffix(ref string) string {
	if idx := strings.LastIndex(ref, ":"); idx > 0 {
		tail := ref[idx+1:]
		if tail == "" {
			return ref
		}
		for _, c := range tail {
			if c < '0' || c > '9' {
				return ref
			}
		}
		return ref[:idx]
	}
	return ref
}
