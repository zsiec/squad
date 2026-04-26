package stats

import "sort"

// computePercentiles returns p50/p90/p99/min/max via linear interpolation
// between adjacent ranks. Empty input → all nil with Count=0.
func computePercentiles(xs []float64) Percentiles {
	n := len(xs)
	if n == 0 {
		return Percentiles{}
	}
	sorted := make([]float64, n)
	copy(sorted, xs)
	sort.Float64s(sorted)
	at := func(q float64) float64 {
		if n == 1 {
			return sorted[0]
		}
		rank := q * float64(n-1)
		lo := int(rank)
		hi := lo + 1
		if hi >= n {
			return sorted[n-1]
		}
		return sorted[lo] + (sorted[hi]-sorted[lo])*(rank-float64(lo))
	}
	p50, p90, p99 := at(0.50), at(0.90), at(0.99)
	min, max := sorted[0], sorted[n-1]
	sum := 0.0
	for _, x := range sorted {
		sum += x
	}
	return Percentiles{P50: &p50, P90: &p90, P99: &p99,
		Min: &min, Max: &max, Sum: &sum, Count: int64(n)}
}
