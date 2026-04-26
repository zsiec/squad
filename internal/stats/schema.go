// Package stats computes operational statistics over the squad operational
// DB: verification rate, claim-duration percentiles, WIP-violation counts,
// reviewer disagreement rate, plus daily series for each. Pure read-side
// aggregation; no new state. Surfaces are squad stats (CLI), /api/stats,
// and /metrics (Prometheus).
package stats

// CurrentSchemaVersion is the on-the-wire version of the stats Snapshot.
// Bump when the shape changes — every json tag is part of the contract;
// renaming or removing one is a breaking change for downstream consumers.
const CurrentSchemaVersion = 1

type Snapshot struct {
	SchemaVersion int          `json:"schema_version"`
	GeneratedAt   int64        `json:"generated_at"`
	RepoID        string       `json:"repo_id"`
	Window        Window       `json:"window"`
	Items         Items        `json:"items"`
	Claims        Claims       `json:"claims"`
	Verification  Verification `json:"verification"`
	Learnings     Learnings    `json:"learnings"`
	Tokens        Tokens       `json:"tokens"`
	ByAgent       []AgentRow   `json:"by_agent"`
	ByEpic        []EpicRow    `json:"by_epic"`
	Series        Series       `json:"series"`
}

type Window struct {
	Since int64  `json:"since"`
	Until int64  `json:"until"`
	Label string `json:"label"`
}

type Items struct {
	Total      int64            `json:"total"`
	Open       int64            `json:"open"`
	Claimed    int64            `json:"claimed"`
	Blocked    int64            `json:"blocked"`
	Done       int64            `json:"done"`
	ByPriority map[string]int64 `json:"by_priority"`
	ByArea     map[string]int64 `json:"by_area"`
}

type Claims struct {
	Active                 int64       `json:"active"`
	CompletedInWindow      int64       `json:"completed_in_window"`
	DurationSeconds        Percentiles `json:"duration_seconds"`
	WallTimeToDoneSeconds  Percentiles `json:"wall_time_to_done_seconds"`
	WIPViolationsAttempted int64       `json:"wip_violations_attempted"`
}

type Percentiles struct {
	P50   *float64 `json:"p50"`
	P90   *float64 `json:"p90"`
	P99   *float64 `json:"p99"`
	Min   *float64 `json:"min"`
	Max   *float64 `json:"max"`
	Sum   *float64 `json:"sum,omitempty"`
	Count int64    `json:"count"`
}

type Verification struct {
	Rate                     *float64                       `json:"rate"`
	DonesWithFullEvidence    int64                          `json:"dones_with_full_evidence"`
	DonesTotal               int64                          `json:"dones_total"`
	ByKind                   map[string]VerificationKindRow `json:"by_kind"`
	ReviewerDisagreementRate *float64                       `json:"reviewer_disagreement_rate"`
	ReviewsWithDisagreement  int64                          `json:"reviews_with_disagreement"`
	ReviewsTotal             int64                          `json:"reviews_total"`
}

type VerificationKindRow struct {
	Attested int64 `json:"attested"`
	Passed   int64 `json:"passed"`
}

type Learnings struct {
	ApprovedTotal          int64    `json:"approved_total"`
	RepeatMistakeRate      *float64 `json:"repeat_mistake_rate"`
	RepeatMistakesInWindow int64    `json:"repeat_mistakes_in_window"`
	NewBugsInWindow        int64    `json:"new_bugs_in_window"`
}

type Tokens struct {
	PerItemEstimateBytes  *float64 `json:"per_item_estimate_bytes"`
	PerItemEstimateMethod string   `json:"per_item_estimate_method"`
	ItemsWithEstimate     int64    `json:"items_with_estimate"`
}

type AgentRow struct {
	AgentID                string   `json:"agent_id"`
	DisplayName            string   `json:"display_name"`
	ClaimsCompleted        int64    `json:"claims_completed"`
	ClaimP50Seconds        *float64 `json:"claim_p50_seconds"`
	ClaimP99Seconds        *float64 `json:"claim_p99_seconds"`
	VerificationRate       *float64 `json:"verification_rate"`
	WIPViolationsAttempted int64    `json:"wip_violations_attempted"`
}

type EpicRow struct {
	Epic             string   `json:"epic"`
	ItemsTotal       int64    `json:"items_total"`
	ItemsDone        int64    `json:"items_done"`
	VerificationRate *float64 `json:"verification_rate"`
}

type Series struct {
	VerificationRateDaily []DailyRatePoint  `json:"verification_rate_daily"`
	ClaimP99Daily         []DailyP99Point   `json:"claim_p99_daily"`
	WIPViolationsDaily    []DailyCountPoint `json:"wip_violations_daily"`
}

type DailyRatePoint struct {
	BucketTS int64   `json:"bucket_ts"`
	Rate     float64 `json:"rate"`
	Count    int64   `json:"count"`
}

type DailyP99Point struct {
	BucketTS   int64   `json:"bucket_ts"`
	P99Seconds float64 `json:"p99_seconds"`
	Count      int64   `json:"count"`
}

type DailyCountPoint struct {
	BucketTS int64 `json:"bucket_ts"`
	Count    int64 `json:"count"`
}
