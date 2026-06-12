package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_IncCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	m.IncCounter("kongming_test_total", map[string]string{"status": "ok"})
	m.IncCounter("kongming_test_total", map[string]string{"status": "ok"})
	m.IncCounter("kongming_test_total", map[string]string{"status": "fail"})

	families, err := reg.Gather()
	require.NoError(t, err)
	var found *dto.MetricFamily
	for _, f := range families {
		if f.GetName() == "kongming_test_total" {
			found = f
			break
		}
	}
	require.NotNil(t, found, "expected metric family kongming_test_total")

	// 按 status 分桶聚合
	statusCount := make(map[string]float64)
	for _, mt := range found.Metric {
		for _, l := range mt.Label {
			if l.GetName() == "status" {
				statusCount[l.GetValue()] = mt.Counter.GetValue()
			}
		}
	}
	assert.Equal(t, 2, int(statusCount["ok"]))
	assert.Equal(t, 1, int(statusCount["fail"]))
}

func TestMetrics_Histogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	m.ObserveHistogram("kongming_test_dur", 0.1, nil)
	m.ObserveHistogram("kongming_test_dur", 0.5, nil)

	families, err := reg.Gather()
	require.NoError(t, err)
	var found *dto.MetricFamily
	for _, f := range families {
		if f.GetName() == "kongming_test_dur_seconds" {
			found = f
			break
		}
	}
	require.NotNil(t, found, "expected metric family kongming_test_dur_seconds with _seconds suffix")
	assert.Equal(t, uint64(2), found.Metric[0].Histogram.GetSampleCount())
}

func TestMetrics_SetGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	m.SetGauge("kongming_active", 7, map[string]string{"kind": "order"})
	m.SetGauge("kongming_active", 3, map[string]string{"kind": "order"})

	families, err := reg.Gather()
	require.NoError(t, err)
	var found *dto.MetricFamily
	for _, f := range families {
		if f.GetName() == "kongming_active" {
			found = f
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, 3.0, found.Metric[0].Gauge.GetValue())
}
