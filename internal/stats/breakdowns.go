package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
)

func computeByAgent(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	rows, err := db.QueryContext(ctx, `
		SELECT ch.agent_id, COALESCE(a.display_name, ch.agent_id),
		       SUM(CASE WHEN ch.outcome = 'done' THEN 1 ELSE 0 END) AS done_count,
		       SUM(CASE WHEN ch.outcome = 'released' THEN 1 ELSE 0 END) AS release_count,
		       GROUP_CONCAT(CASE WHEN ch.outcome = 'done'
		                         THEN ch.released_at - ch.claimed_at END)
		FROM claim_history ch
		LEFT JOIN agents a ON a.id = ch.agent_id
		WHERE ch.repo_id = ? AND ch.outcome IN ('done', 'released')
		  AND ch.released_at >= ? AND (? = 0 OR ch.released_at < ?)
		GROUP BY ch.agent_id`, repoID, since, until, until)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasAttestations := tableExists(ctx, db, "attestations")
	out := []AgentRow{}
	for rows.Next() {
		var ar AgentRow
		var concat sql.NullString
		if err := rows.Scan(&ar.AgentID, &ar.DisplayName,
			&ar.ClaimsCompleted, &ar.ReleaseCount, &concat); err != nil {
			return err
		}
		if ar.ReleaseCount > 0 {
			r := float64(ar.ClaimsCompleted) / float64(ar.ReleaseCount)
			ar.Ratio = &r
		}
		if concat.Valid {
			p := computePercentiles(splitInts(concat.String))
			ar.ClaimP50Seconds, ar.ClaimP99Seconds = p.P50, p.P99
		}
		if hasAttestations {
			ar.VerificationRate = perAgentVerificationRate(ctx, db, repoID, ar.AgentID,
				since, until, ar.ClaimsCompleted)
		}
		ar.WIPViolationsAttempted, _ = CountWIPViolationsByAgent(ctx, db, repoID, ar.AgentID, since, until)
		out = append(out, ar)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ClaimsCompleted != out[j].ClaimsCompleted {
			return out[i].ClaimsCompleted > out[j].ClaimsCompleted
		}
		return out[i].AgentID < out[j].AgentID
	})
	const cap = 50
	if len(out) > cap {
		spill := AgentRow{AgentID: "_other", DisplayName: "_other"}
		for _, a := range out[cap:] {
			spill.ClaimsCompleted += a.ClaimsCompleted
			spill.ReleaseCount += a.ReleaseCount
			spill.WIPViolationsAttempted += a.WIPViolationsAttempted
		}
		if spill.ReleaseCount > 0 {
			r := float64(spill.ClaimsCompleted) / float64(spill.ReleaseCount)
			spill.Ratio = &r
		}
		out = append(out[:cap], spill)
	}
	snap.ByAgent = out
	return nil
}

// perAgentVerificationRate runs the dones-with-full-evidence query scoped to
// one agent. Returns nil if the agent has zero completed claims in window.
func perAgentVerificationRate(ctx context.Context, db *sql.DB, repoID, agentID string, since, until, completed int64) *float64 {
	if completed == 0 {
		return nil
	}
	var full int64
	_ = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT ch.item_id) FROM claim_history ch
		WHERE ch.repo_id = ? AND ch.agent_id = ? AND ch.outcome = 'done'
		  AND ch.released_at >= ? AND (? = 0 OR ch.released_at < ?)
		  AND EXISTS (SELECT 1 FROM attestations a WHERE a.repo_id = ch.repo_id
		              AND a.item_id = ch.item_id AND a.kind='test' AND a.exit_code=0)
		  AND EXISTS (SELECT 1 FROM attestations a WHERE a.repo_id = ch.repo_id
		              AND a.item_id = ch.item_id AND a.kind='review' AND a.exit_code=0)`,
		repoID, agentID, since, until, until).Scan(&full)
	r := float64(full) / float64(completed)
	return &r
}

func computeByEpic(ctx context.Context, db *sql.DB, repoID string, _, _ int64, snap *Snapshot) error {
	if !columnExists(ctx, db, "items", "epic_id") {
		snap.ByEpic = []EpicRow{}
		return nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(epic_id, ''), COUNT(*),
		       SUM(CASE WHEN status='done' THEN 1 ELSE 0 END)
		FROM items WHERE repo_id = ? AND archived = 0
		GROUP BY COALESCE(epic_id, '')`, repoID)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasAttestations := tableExists(ctx, db, "attestations")
	out := []EpicRow{}
	for rows.Next() {
		var er EpicRow
		if err := rows.Scan(&er.Epic, &er.ItemsTotal, &er.ItemsDone); err != nil {
			return err
		}
		if er.Epic == "" {
			continue
		}
		if hasAttestations && er.ItemsDone > 0 {
			er.VerificationRate = perEpicVerificationRate(ctx, db, repoID, er.Epic, er.ItemsDone)
		}
		out = append(out, er)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ItemsDone != out[j].ItemsDone {
			return out[i].ItemsDone > out[j].ItemsDone
		}
		return out[i].Epic < out[j].Epic
	})
	snap.ByEpic = out
	return nil
}

func perEpicVerificationRate(ctx context.Context, db *sql.DB, repoID, epic string, done int64) *float64 {
	var full int64
	_ = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT i.item_id) FROM items i
		WHERE i.repo_id = ? AND i.epic_id = ? AND i.status = 'done'
		  AND EXISTS (SELECT 1 FROM attestations a WHERE a.repo_id = i.repo_id
		              AND a.item_id = i.item_id AND a.kind='test' AND a.exit_code=0)
		  AND EXISTS (SELECT 1 FROM attestations a WHERE a.repo_id = i.repo_id
		              AND a.item_id = i.item_id AND a.kind='review' AND a.exit_code=0)`,
		repoID, epic).Scan(&full)
	r := float64(full) / float64(done)
	return &r
}

// computeByCapability counts done items in window per requires_capability
// tag. Multi-tag items increment each tag once. Empty tag-set items land
// under "(untagged)". Done-ness is sourced from claim_history (so the
// since/until window applies); the items.requires_capability column is
// joined for the tag set. No-op when the column is absent.
func computeByCapability(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	snap.ByCapability = []CapabilityRow{}
	if !columnExists(ctx, db, "items", "requires_capability") {
		return nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT i.item_id, COALESCE(i.requires_capability, '[]')
		FROM items i
		INNER JOIN claim_history ch ON ch.item_id = i.item_id AND ch.repo_id = i.repo_id
		WHERE i.repo_id = ? AND ch.outcome = 'done'
		  AND ch.released_at >= ? AND (? = 0 OR ch.released_at < ?)`,
		repoID, since, until, until,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var itemID, raw string
		if err := rows.Scan(&itemID, &raw); err != nil {
			return err
		}
		var tags []string
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &tags)
		}
		if len(tags) == 0 {
			counts["(untagged)"]++
			continue
		}
		for _, tg := range tags {
			counts[tg]++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	out := make([]CapabilityRow, 0, len(counts))
	for k, v := range counts {
		out = append(out, CapabilityRow{Capability: k, DoneCount: v})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DoneCount != out[j].DoneCount {
			return out[i].DoneCount > out[j].DoneCount
		}
		return out[i].Capability < out[j].Capability
	})
	snap.ByCapability = out
	return nil
}

func columnExists(ctx context.Context, db *sql.DB, table, col string) bool {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil && name == col {
			return true
		}
	}
	return false
}

// splitInts parses GROUP_CONCAT output ("100,200,300") into []float64. Tolerant
// of leading minus signs; ignores anything else.
func splitInts(csv string) []float64 {
	if csv == "" {
		return nil
	}
	out := make([]float64, 0, 16)
	cur, neg, parsing := 0, false, false
	for i := 0; i <= len(csv); i++ {
		if i == len(csv) || csv[i] == ',' {
			if parsing {
				v := float64(cur)
				if neg {
					v = -v
				}
				out = append(out, v)
			}
			cur, neg, parsing = 0, false, false
			continue
		}
		c := csv[i]
		switch {
		case c == '-':
			neg, parsing = true, true
		case c >= '0' && c <= '9':
			cur, parsing = cur*10+int(c-'0'), true
		}
	}
	return out
}
