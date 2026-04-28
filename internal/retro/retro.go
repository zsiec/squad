// Package retro generates a weekly markdown digest of failure modes
// and process suggestions from the squad ledger. It is read-only on
// the database and produces deterministic output for fixed input, so
// it is safe to run on a cron and to assert against in tests.
package retro

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Period names a one-week ISO-8601 window. Since is inclusive (Mon
// 00:00 UTC), Until is exclusive (next Mon 00:00 UTC).
type Period struct {
	Year  int
	Week  int
	Since int64
	Until int64
}

// String returns the canonical "YYYY-WNN" label used in filenames.
func (p Period) String() string {
	return fmt.Sprintf("%04d-W%02d", p.Year, p.Week)
}

// Opts parameterises Generate. RepoID is required (empty would mean
// workspace mode, which the retro does not support — a per-repo
// digest is more actionable than a global one).
type Opts struct {
	RepoID   string
	Period   Period
	MinItems int
}

const defaultMinItems = 5

const insufficientSignal = "insufficient signal — fewer than the configured threshold of items closed in the period.\n"

// CurrentWeek returns the ISO-week period containing now, normalized
// to UTC. The week starts on the Monday and ends at the next Monday.
func CurrentWeek(now time.Time) Period {
	utc := now.UTC()
	year, week := utc.ISOWeek()
	monday := isoWeekMonday(year, week)
	return Period{
		Year:  year,
		Week:  week,
		Since: monday.Unix(),
		Until: monday.Add(7 * 24 * time.Hour).Unix(),
	}
}

// ParseWeek parses "YYYY-WNN" or "YYYY-NN" labels. Both forms are
// accepted because users (and shells) drop the W often. Returns an
// error for empty input, missing or non-numeric components, weeks
// outside [1, 53], or years outside [2000, 9999] (a sanity range —
// not a hard limit, but stops obvious typos).
func ParseWeek(s string) (Period, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Period{}, errors.New("empty week string")
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return Period{}, fmt.Errorf("invalid week %q: expected YYYY-WNN or YYYY-NN", s)
	}
	year, err := atoiStrict(parts[0])
	if err != nil {
		return Period{}, fmt.Errorf("invalid year in %q: %w", s, err)
	}
	if year < 2000 || year > 9998 {
		return Period{}, fmt.Errorf("year %d out of sane range", year)
	}
	weekPart := strings.TrimPrefix(parts[1], "W")
	weekPart = strings.TrimPrefix(weekPart, "w")
	week, err := atoiStrict(weekPart)
	if err != nil {
		return Period{}, fmt.Errorf("invalid week in %q: %w", s, err)
	}
	if week < 1 || week > 53 {
		return Period{}, fmt.Errorf("week %d out of range", week)
	}
	monday := isoWeekMonday(year, week)
	gotYear, gotWeek := monday.ISOWeek()
	if gotYear != year || gotWeek != week {
		return Period{}, fmt.Errorf("week %d-W%d does not exist (canonical: %d-W%d)", year, week, gotYear, gotWeek)
	}
	return Period{
		Year:  year,
		Week:  week,
		Since: monday.Unix(),
		Until: monday.Add(7 * 24 * time.Hour).Unix(),
	}, nil
}

func atoiStrict(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-numeric: %q", s)
		}
	}
	var n int
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// isoWeekMonday returns the UTC Monday that begins ISO-year/week.
func isoWeekMonday(year, week int) time.Time {
	// ISO week 1 contains the year's first Thursday. Walk from Jan 4
	// back to its Monday — that's week 1's Monday by definition — then
	// advance (week-1)*7 days.
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	wd := int(jan4.Weekday())
	if wd == 0 {
		wd = 7
	}
	mondayWeek1 := jan4.AddDate(0, 0, -(wd - 1))
	return mondayWeek1.AddDate(0, 0, (week-1)*7)
}

// Generate runs the retro queries against db and returns the
// markdown body. ok=false signals "not enough closed items in the
// period — write a placeholder file instead".
func Generate(ctx context.Context, db *sql.DB, opts Opts) (string, bool, error) {
	min := opts.MinItems
	if min <= 0 {
		min = defaultMinItems
	}
	closes, err := readCloses(ctx, db, opts.RepoID, opts.Period)
	if err != nil {
		return "", false, err
	}
	failingByKind, err := readFailingAttests(ctx, db, opts.RepoID, opts.Period)
	if err != nil {
		return "", false, err
	}
	// Threshold counts CI thrash too: a week with zero closes but lots
	// of failing attestations is still a real retro signal — that's
	// precisely the "test-attestation kinds with high failure rate"
	// example the AC names. Suppressing it would hide the most
	// actionable case.
	signal := len(closes)
	for _, k := range failingByKind {
		signal += k.fail
	}
	if signal < min {
		return renderInsufficient(opts.Period), false, nil
	}
	return renderRetro(opts.Period, closes, failingByKind), true, nil
}

type closeRow struct {
	itemID    string
	itemType  string
	itemPrio  string
	itemArea  string
	status    string
	outcome   string
	durationS int64
}

func readCloses(ctx context.Context, db *sql.DB, repoID string, p Period) ([]closeRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ch.item_id,
		       COALESCE(i.type, ''), COALESCE(i.priority, ''), COALESCE(i.area, ''),
		       COALESCE(i.status, ''),
		       ch.outcome,
		       (ch.released_at - ch.claimed_at)
		FROM claim_history ch
		LEFT JOIN items i ON i.repo_id = ch.repo_id AND i.item_id = ch.item_id
		WHERE ch.repo_id = ? AND ch.released_at >= ? AND ch.released_at < ?
		ORDER BY ch.released_at ASC, ch.id ASC
	`, repoID, p.Since, p.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []closeRow
	for rows.Next() {
		var r closeRow
		if err := rows.Scan(&r.itemID, &r.itemType, &r.itemPrio, &r.itemArea, &r.status, &r.outcome, &r.durationS); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type kindFailure struct {
	kind  string
	total int
	fail  int
}

func readFailingAttests(ctx context.Context, db *sql.DB, repoID string, p Period) ([]kindFailure, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT kind,
		       COUNT(*),
		       SUM(CASE WHEN exit_code != 0 THEN 1 ELSE 0 END)
		FROM attestations
		WHERE repo_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY kind
		ORDER BY kind ASC
	`, repoID, p.Since, p.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []kindFailure
	for rows.Next() {
		var k kindFailure
		var fail sql.NullInt64
		if err := rows.Scan(&k.kind, &k.total, &fail); err != nil {
			return nil, err
		}
		k.fail = int(fail.Int64)
		out = append(out, k)
	}
	return out, rows.Err()
}

func renderInsufficient(p Period) string {
	return fmt.Sprintf("# Retro %s\n\n%s", p, insufficientSignal)
}

func renderRetro(p Period, closes []closeRow, fails []kindFailure) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Retro %s\n\n", p)
	fmt.Fprintf(&sb, "Period: %s → %s (UTC). %d closed item-claims in window.\n\n",
		time.Unix(p.Since, 0).UTC().Format("2006-01-02"),
		time.Unix(p.Until, 0).UTC().Format("2006-01-02"),
		len(closes))

	modes := topFailureModes(closes, fails)
	sb.WriteString("## Top failure modes\n\n")
	if len(modes) == 0 {
		sb.WriteString("- (none surfaced — clean week)\n\n")
	} else {
		for _, m := range modes {
			fmt.Fprintf(&sb, "- %s (%d)\n", m.label, m.count)
		}
		sb.WriteString("\n")
	}

	slowType, slowMedian, allMedian := slowestType(closes)
	sb.WriteString("## Slowest item type\n\n")
	if slowType == "" {
		sb.WriteString("- (no typed closes in window)\n\n")
	} else {
		fmt.Fprintf(&sb, "- `%s`: median %s (overall median %s)\n\n",
			slowType, formatDur(slowMedian), formatDur(allMedian))
	}

	sb.WriteString("## Recommendation\n\n")
	sb.WriteString(recommend(closes, modes, slowType, slowMedian, allMedian))
	sb.WriteString("\n")

	return sb.String()
}

type failureMode struct {
	label string
	count int
}

func topFailureModes(closes []closeRow, fails []kindFailure) []failureMode {
	candidates := []failureMode{}

	blocked := 0
	released := 0
	for _, c := range closes {
		switch {
		case strings.EqualFold(c.status, "blocked"):
			blocked++
		case c.outcome == "released" || c.outcome == "force_released":
			released++
		}
	}
	if blocked > 0 {
		candidates = append(candidates, failureMode{
			label: "items closed as `blocked`",
			count: blocked,
		})
	}
	if released > 0 {
		candidates = append(candidates, failureMode{
			label: "claims released without `done` (abandoned or handed off)",
			count: released,
		})
	}
	for _, k := range fails {
		if k.fail == 0 {
			continue
		}
		candidates = append(candidates, failureMode{
			label: fmt.Sprintf("attestation kind `%s` failures", k.kind),
			count: k.fail,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count > candidates[j].count
		}
		return candidates[i].label < candidates[j].label
	})
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates
}

func slowestType(closes []closeRow) (typeName string, slowMedian, allMedian time.Duration) {
	if len(closes) == 0 {
		return "", 0, 0
	}
	byType := map[string][]int64{}
	var all []int64
	for _, c := range closes {
		if c.itemType == "" || c.outcome != "done" {
			continue
		}
		byType[c.itemType] = append(byType[c.itemType], c.durationS)
		all = append(all, c.durationS)
	}
	if len(byType) == 0 {
		return "", 0, 0
	}
	allMedianSec := medianInt64(all)
	allMedian = time.Duration(allMedianSec) * time.Second

	var typeNames []string
	for t := range byType {
		typeNames = append(typeNames, t)
	}
	sort.Strings(typeNames)

	var pickedMedian int64
	for _, t := range typeNames {
		m := medianInt64(byType[t])
		if m > pickedMedian {
			pickedMedian = m
			typeName = t
		}
	}
	slowMedian = time.Duration(pickedMedian) * time.Second
	return
}

func medianInt64(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	cp := make([]int64, len(xs))
	copy(cp, xs)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

func formatDur(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 48*time.Hour {
		mins := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), mins)
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func recommend(closes []closeRow, modes []failureMode, slowType string, slowMedian, allMedian time.Duration) string {
	dones := 0
	for _, c := range closes {
		if c.outcome == "done" {
			dones++
		}
	}
	if len(closes) > 0 {
		// Rule 1: lots of releases-without-done.
		notDone := len(closes) - dones
		if notDone*100 >= 30*len(closes) && notDone >= 2 {
			return "Many claims released without close-out — split items smaller before claim, or finish blockers before parking.\n"
		}
	}
	// Rule 2: outlier slow type.
	if slowType != "" && allMedian > 0 && slowMedian >= 2*allMedian {
		return fmt.Sprintf("Type `%s` took ~%s vs overall median %s — consider decomposition before claim for that type.\n",
			slowType, formatDur(slowMedian), formatDur(allMedian))
	}
	// Rule 3: failing-attestation kind dominates.
	if len(modes) > 0 && strings.HasPrefix(modes[0].label, "attestation kind") {
		return "Verification gates fired more than other signals this week — ensure local lint/test runs precede `squad attest`.\n"
	}
	return "No outlier patterns detected — keep current cadence.\n"
}
