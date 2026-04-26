package hygiene

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CheckStaleCaptures flags captured items whose captured_at is older than
// threshold. A pile of months-old captures is a sign nobody is triaging the
// inbox — surface it instead of letting them rot silently.
func CheckStaleCaptures(ctx context.Context, db *sql.DB, repoID string, threshold time.Duration) []Finding {
	cutoff := time.Now().Add(-threshold).Unix()
	rows, err := db.QueryContext(ctx, `
		SELECT item_id, captured_at FROM items
		WHERE repo_id = ? AND status = 'captured'
		  AND captured_at IS NOT NULL AND captured_at < ?
	`, repoID, cutoff)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var id string
		var capAt int64
		if err := rows.Scan(&id, &capAt); err != nil {
			continue
		}
		ageDays := int(time.Since(time.Unix(capAt, 0)).Hours() / 24)
		out = append(out, Finding{
			Severity: SeverityWarn,
			Code:     "stale_capture",
			Message:  fmt.Sprintf("%s captured %d days ago; reject or commit to it", id, ageDays),
			Fix:      "squad accept " + id + "  OR  squad reject " + id + " --reason \"...\"",
		})
	}
	return out
}

// CheckInboxOverflow flags an inbox larger than threshold (strict >). If the
// pile of captured items grows unbounded, intake is broken — flag it.
func CheckInboxOverflow(ctx context.Context, db *sql.DB, repoID string, threshold int) []Finding {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM items WHERE repo_id = ? AND status = 'captured'`,
		repoID).Scan(&n); err != nil {
		return nil
	}
	if n <= threshold {
		return nil
	}
	return []Finding{{
		Severity: SeverityWarn,
		Code:     "inbox_overflow",
		Message:  fmt.Sprintf("inbox has %d captured items (>%d); triage backlog overflowing", n, threshold),
		Fix:      "squad inbox  — review and accept or reject pending items",
	}}
}

// CheckRejectedLogSize flags .squad/rejected.log when its line count exceeds
// threshold. A bloated rejection log usually means capture quality is poor.
func CheckRejectedLogSize(squadDir string, threshold int) []Finding {
	p := filepath.Join(squadDir, "rejected.log")
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return nil
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		n++
	}
	if n <= threshold {
		return nil
	}
	return []Finding{{
		Severity: SeverityInfo,
		Code:     "rejected_log_overflow",
		Message:  fmt.Sprintf("%s has %d entries (>%d); capture discipline may be lax", p, n, threshold),
		Fix:      "review .squad/rejected.log and tighten what gets captured",
	}}
}
