package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_BasicRegistration(t *testing.T) {
	tests := []struct{ name string }{
		{name: "registered"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if AllocationDuration == nil {
				t.Fatalf("AllocationDuration is nil")
			}
			if AllocationsTotal == nil {
				t.Fatalf("AllocationsTotal is nil")
			}
		})
	}
}

func TestMetrics_AllocationsTotal(t *testing.T) {
	tests := []struct {
		name  string
		label string
		incN  int
	}{
		{name: "success label", label: "Success", incN: 1},
		{name: "failure label", label: "Failure", incN: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := testutil.ToFloat64(AllocationsTotal.WithLabelValues(tt.label))
			for i := 0; i < tt.incN; i++ {
				AllocationsTotal.WithLabelValues(tt.label).Inc()
			}
			after := testutil.ToFloat64(AllocationsTotal.WithLabelValues(tt.label))
			diff := after - before
			if diff != float64(tt.incN) {
				t.Fatalf("counter diff mismatch\nexpected: %#v\nactual: %#v", float64(tt.incN), diff)
			}
		})
	}
}

func TestMetrics_AllocationDuration(t *testing.T) {
	tests := []struct {
		name    string
		observe float64
	}{
		{name: "small", observe: 0.1},
		{name: "large", observe: 3.2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AllocationDuration.Observe(tt.observe)
			count := testutil.CollectAndCount(AllocationDuration)
			assert.Greater(t, count, 0, "histogram not collected; count=%#v", count)
		})
	}
}
