package oplog

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntervalMaxMetric(t *testing.T) {
	t.Run("basic test", func(t *testing.T) {
		t.Parallel()

		m := NewIntervalMaxMetric(IntervalMaxOpts{
			Opts:           prometheus.Opts{},
			ReportInterval: 0,
		}, []string{"l1", "l2"}, []string{"a", "test"})

		req := require.New(t)
		asrt := assert.New(t)

		req.Nil(m.currentMax)
		req.Nil(m.previousMax)

		req.Equal(m.opts.ReportInterval, DefaultInterval)

		m.Report(12.)

		req.NotNil(m.currentMax)
		req.Nil(m.previousMax)

		asrt.Equal(m.currentMax.value, 12.)

		m.Report(13.)

		req.Nil(m.previousMax)
		asrt.Equal(m.currentMax.value, 13.)

		collCh := make(chan prometheus.Metric, 1)
		m.Collect(collCh)

		req.Len(collCh, 0)
	})

	t.Run("output once", func(t *testing.T) {
		t.Parallel()

		const interval = 5 * time.Millisecond

		m := NewIntervalMaxMetric(IntervalMaxOpts{
			Opts:           prometheus.Opts{},
			ReportInterval: interval,
		}, []string{"l1", "l2"}, []string{"a", "test"})

		req := require.New(t)
		asrt := assert.New(t)

		m.Report(12.)
		m.Report(13.)
		m.Report(2300.)

		collCh := make(chan prometheus.Metric, 1)
		m.Collect(collCh)

		req.Len(collCh, 0)

		time.Sleep(interval)

		m.Collect(collCh)
		req.Len(collCh, 1)
		req.Nil(m.currentMax)
		req.NotNil(m.previousMax)

		val := <-collCh

		dt := dto.Metric{}

		req.NoError(val.Write(&dt))
		req.NotNil(dt.Gauge)
		req.NotNil(dt.Label)

		l1 := "l1"
		l2 := "l2"

		v1 := "a"
		v2 := "test"

		asrt.Equal([]*dto.LabelPair{
			{
				Name:  &l1,
				Value: &v1,
			},
			{
				Name:  &l2,
				Value: &v2,
			},
		}, dt.Label)

		asrt.Equal(2300., *dt.Gauge.Value)

		time.Sleep(interval)

		m.Collect(collCh)
		req.Len(collCh, 0)
	})

	t.Run("output multiple", func(t *testing.T) {
		t.Parallel()

		const interval = 5 * time.Millisecond

		m := NewIntervalMaxMetric(IntervalMaxOpts{
			Opts:           prometheus.Opts{},
			ReportInterval: interval,
		}, []string{"l1", "l2"}, []string{"a", "test"})

		req := require.New(t)
		asrt := assert.New(t)

		m.Report(12)
		m.Report(13)
		m.Report(14)
		m.Report(2)
		m.Report(0)
		m.Report(-29)
		m.Report(12)

		c := make(chan prometheus.Metric, 1)
		m.Collect(c)
		req.Len(c, 0)

		time.Sleep(interval)
		m.Collect(c)
		req.Len(c, 1)

		asrt.Equal(14., val(t, <-c))

		time.Sleep(interval)
		m.Collect(c)
		req.Len(c, 0)

		m.Report(52)
		m.Report(-12)
		m.Report(0)
		m.Report(512395)
		m.Report(18)

		time.Sleep(interval)
		m.Collect(c)
		req.Len(c, 1)

		asrt.Equal(512395., val(t, <-c))

		m.Report(0)

		time.Sleep(interval)
		m.Collect(c)
		req.Len(c, 1)
		asrt.Equal(0., val(t, <-c))

		m.Report(-1)

		time.Sleep(interval)
		m.Collect(c)
		req.Len(c, 1)
		asrt.Equal(-1., val(t, <-c))
	})
}

func val(t *testing.T, metric prometheus.Metric) float64 {
	d := dto.Metric{}
	require.NoError(t, metric.Write(&d))

	return *d.Gauge.Value
}

func labels(t *testing.T, metric prometheus.Metric) map[string]string {
	d := dto.Metric{}
	require.NoError(t, metric.Write(&d))

	ret := map[string]string{}

	for _, pair := range d.Label {
		ret[*pair.Name] = *pair.Value
	}

	return ret
}

func TestIntervalMaxMetricVec(t *testing.T) {
	t.Parallel()

	const interval = 5 * time.Millisecond

	req := require.New(t)
	asrt := assert.New(t)

	m := NewIntervalMaxMetricVec(IntervalMaxVecOpts{
		IntervalMaxOpts: IntervalMaxOpts{
			Opts:           prometheus.Opts{},
			ReportInterval: interval,
		},
		GCInterval: interval,
	}, []string{"l1", "l2"})

	m.Report(12, "a", "test")
	m.Report(0, "another", "test")
	m.Report(13, "a", "test")
	m.Report(-1, "another", "test")

	c := make(chan prometheus.Metric, 16)

	m.Collect(c)
	req.Len(c, 0)

	time.Sleep(interval)

	m.Collect(c)
	req.Len(c, 2)

	for i := 0; i < 2; i++ {
		v := <-c

		l := labels(t, v)
		value := val(t, v)

		if l["l1"] == "a" {
			asrt.Equal(13., value)
		} else {
			asrt.Equal(0., value)
		}
	}

	time.Sleep(interval)

	// ensure a gc happens
	count := 0
	m.mp.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	asrt.Equal(2, count)

	m.Collect(c)

	count = 0
	m.mp.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	asrt.Equal(0, count)
}
