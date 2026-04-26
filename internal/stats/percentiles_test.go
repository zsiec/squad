package stats

import (
	"math"
	"testing"
)

func TestPercentilesEmpty(t *testing.T) {
	got := computePercentiles(nil)
	if got.Count != 0 || got.P50 != nil || got.P90 != nil || got.P99 != nil {
		t.Errorf("empty: %+v", got)
	}
	if got.Sum != nil {
		t.Errorf("empty: sum=%v want nil", got.Sum)
	}
}

func TestPercentilesSingle(t *testing.T) {
	got := computePercentiles([]float64{42})
	for name, p := range map[string]*float64{
		"p50": got.P50, "p90": got.P90, "p99": got.P99,
		"min": got.Min, "max": got.Max,
	} {
		if p == nil || *p != 42 {
			t.Errorf("%s: %v", name, p)
		}
	}
	if got.Sum == nil || *got.Sum != 42 {
		t.Errorf("sum: %v want 42", got.Sum)
	}
}

func TestPercentilesLinearInterpolation(t *testing.T) {
	xs := make([]float64, 101)
	for i := range xs {
		xs[i] = float64(i)
	}
	got := computePercentiles(xs)
	for _, c := range []struct {
		name string
		p    *float64
		want float64
	}{
		{"p50", got.P50, 50}, {"p90", got.P90, 90}, {"p99", got.P99, 99},
		{"min", got.Min, 0}, {"max", got.Max, 100},
	} {
		if c.p == nil || math.Abs(*c.p-c.want) > 1e-9 {
			t.Errorf("%s: got %v want %v", c.name, c.p, c.want)
		}
	}
	if got.Sum == nil || math.Abs(*got.Sum-5050.0) > 1e-9 {
		t.Errorf("sum: got %v want 5050", got.Sum)
	}
}

func TestPercentilesUnsorted(t *testing.T) {
	got := computePercentiles([]float64{99, 1, 50, 5, 90})
	if got.P50 == nil || *got.P50 != 50 || *got.Min != 1 {
		t.Errorf("unsorted: p50=%v min=%v", got.P50, got.Min)
	}
	if got.Sum == nil || *got.Sum != 245.0 {
		t.Errorf("sum: %v want 245", got.Sum)
	}
}
