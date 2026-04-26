package server

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/zsiec/squad/internal/stats"
)

var (
	descItemsTotal = prometheus.NewDesc("squad_items_total",
		"Items by status, snapshot.", []string{"repo", "status"}, nil)
	descClaimDur = prometheus.NewDesc("squad_claim_duration_seconds",
		"Claim hold duration percentiles.", []string{"repo"}, nil)
	descVerify = prometheus.NewDesc("squad_verification_rate",
		"Fraction of dones with full evidence chain.", []string{"repo"}, nil)
	descDisagree = prometheus.NewDesc("squad_reviewer_disagreement_rate",
		"Fraction of reviews with disagreement.", []string{"repo"}, nil)
	descWIP = prometheus.NewDesc("squad_wip_violations_attempted_total",
		"Cumulative claim attempts that hit the WIP cap.", []string{"repo"}, nil)
	descRepeat = prometheus.NewDesc("squad_repeat_mistake_rate",
		"Fraction of new bugs whose area matches an approved learning.", []string{"repo"}, nil)
	descAttest = prometheus.NewDesc("squad_attestations_total",
		"Attestation rows by kind and status.", []string{"repo", "kind", "status"}, nil)
)

func (s *Server) prometheusHandler() http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(&serverCollector{srv: s})
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

type serverCollector struct{ srv *Server }

func (c *serverCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{descItemsTotal, descClaimDur,
		descVerify, descDisagree, descWIP, descRepeat, descAttest} {
		ch <- d
	}
}

func (c *serverCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := stats.Compute(ctx, c.srv.db, stats.ComputeOpts{
		RepoID: c.srv.cfg.RepoID, Window: 24 * time.Hour,
	})
	if err != nil {
		return
	}
	repo := snap.RepoID
	g := func(d *prometheus.Desc, v float64, lvs ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v,
			append([]string{repo}, lvs...)...)
	}
	cnt := func(d *prometheus.Desc, v float64, lvs ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v,
			append([]string{repo}, lvs...)...)
	}
	g(descItemsTotal, float64(snap.Items.Open), "open")
	g(descItemsTotal, float64(snap.Items.Claimed), "claimed")
	g(descItemsTotal, float64(snap.Items.Blocked), "blocked")
	g(descItemsTotal, float64(snap.Items.Done), "done")
	if p := snap.Claims.DurationSeconds; p.Count > 0 {
		q := map[float64]float64{}
		if p.P50 != nil {
			q[0.5] = *p.P50
		}
		if p.P90 != nil {
			q[0.9] = *p.P90
		}
		if p.P99 != nil {
			q[0.99] = *p.P99
		}
		// Sum is approximated from min/max because exact would require an
		// extra DB query — the percentiles are the load-bearing values here.
		var sum float64
		if p.Min != nil && p.Max != nil {
			sum = (*p.Min + *p.Max) * float64(p.Count) / 2
		}
		ch <- prometheus.MustNewConstSummary(descClaimDur, uint64(p.Count), sum, q, repo)
	}
	if r := snap.Verification.Rate; r != nil {
		g(descVerify, *r)
	}
	if r := snap.Verification.ReviewerDisagreementRate; r != nil {
		g(descDisagree, *r)
	}
	cnt(descWIP, float64(snap.Claims.WIPViolationsAttempted))
	if r := snap.Learnings.RepeatMistakeRate; r != nil {
		g(descRepeat, *r)
	}
	for kind, row := range snap.Verification.ByKind {
		cnt(descAttest, float64(row.Passed), kind, "pass")
		cnt(descAttest, float64(row.Attested-row.Passed), kind, "fail")
	}
}
