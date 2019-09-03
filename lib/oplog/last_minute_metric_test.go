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

		m := NewIntervalMaxMetric(IntervalMaxOpts{
			Opts:           prometheus.Opts{},
			ReportInterval: 500 * time.Millisecond,
		}, []string{"l1", "l2"}, []string{"a", "test"})

		req := require.New(t)
		asrt := assert.New(t)

		startTime := time.Now()
		m.Report(12.)
		m.Report(13.)
		m.Report(2300.)

		collCh := make(chan prometheus.Metric, 1)
		m.Collect(collCh)

		req.Len(collCh, 0)

		req.True(time.Since(startTime) < 500*time.Millisecond)
		time.Sleep(time.Until(startTime.Add(500 * time.Millisecond)))

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
	})
}
