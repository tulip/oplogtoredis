package oplog

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type IntervalMaxMetric struct {
	desc        *prometheus.Desc
	opts        IntervalMaxOpts
	labelValues []string

	lck sync.Mutex

	currentMax  *lastMax
	previousMax *lastMax
}

type lastMax struct {
	value        float64
	bucketedTime time.Time
}

const DefaultInterval = 1 * time.Minute

type IntervalMaxOpts struct {
	prometheus.Opts

	// ReportInterval is the interval by which reports will be bucketed. Default 1m.
	ReportInterval time.Duration
}

func NewIntervalMaxMetric(opts IntervalMaxOpts, labels []string, labelValues []string) *IntervalMaxMetric {
	if opts.ReportInterval == 0 {
		opts.ReportInterval = DefaultInterval
	}

	return &IntervalMaxMetric{
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(
				opts.Namespace, opts.Subsystem, opts.Name,
			),
			opts.Help,
			labels,
			opts.ConstLabels,
		),
		opts:        opts,
		labelValues: labelValues,

		currentMax: nil,
	}
}

func (c *IntervalMaxMetric) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.desc
}

func (c *IntervalMaxMetric) Collect(mtcs chan<- prometheus.Metric) {
	c.lck.Lock()
	defer c.lck.Unlock()

	currentBucket := c.thisTimeBucket()
	c.rotate(currentBucket)

	if c.previousMax == nil {
		return
	}

	if currentBucket.Sub(c.previousMax.bucketedTime) != c.opts.ReportInterval {
		return
	}

	mtcs <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, c.previousMax.value, c.labelValues...)
}

func (c *IntervalMaxMetric) Report(value float64) {
	c.lck.Lock()
	defer c.lck.Unlock()

	thisTimeBucket := c.thisTimeBucket()
	c.rotate(thisTimeBucket)

	maxVal := &lastMax{
		value:        value,
		bucketedTime: thisTimeBucket,
	}

	if c.currentMax == nil {
		c.currentMax = maxVal
		return
	}

	if thisTimeBucket.Equal(c.currentMax.bucketedTime) && c.currentMax.value < value {
		c.currentMax = maxVal
		return
	}

	if thisTimeBucket.After(c.currentMax.bucketedTime) {
		c.currentMax = maxVal
		return
	} else { // this bucket is before previous bucket
		panic("interval max metric time traveled")
	}
}

// pre: c is locked
func (c *IntervalMaxMetric) rotate(timeBucket time.Time) {
	if c.currentMax == nil {
		return
	}

	if !timeBucket.After(c.currentMax.bucketedTime) {
		return
	}

	c.previousMax = c.currentMax
	c.currentMax = nil
}

func (c *IntervalMaxMetric) thisTimeBucket() time.Time {
	return time.Now().Truncate(c.opts.ReportInterval)
}

type IntervalMaxMetricVec struct {
	mp     sync.Map
	labels []string
	desc   *prometheus.Desc
	opts   IntervalMaxOpts
}

func NewIntervalMaxMetricVec(opts IntervalMaxOpts, labels []string) *IntervalMaxMetricVec {
	return &IntervalMaxMetricVec{
		labels: labels,
		opts:   opts,
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
			opts.Help,
			labels,
			opts.ConstLabels,
		),
	}
}

func (c *IntervalMaxMetricVec) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.desc
}

func (c *IntervalMaxMetricVec) Collect(coll chan<- prometheus.Metric) {
	c.mp.Range(func(_, v interface{}) bool {
		v.(*IntervalMaxMetric).Collect(coll)
		return true
	})
}

func (c *IntervalMaxMetricVec) Report(value float64, labelValues ...string) {
	key := labelKey(labelValues)

	m, ok := c.mp.Load(key)
	if !ok {
		m, _ = c.mp.LoadOrStore(key, NewIntervalMaxMetric(c.opts, c.labels, labelValues))
	}

	m.(*IntervalMaxMetric).Report(value)
}

func labelKey(labels []string) string {
	return "imv::" + strings.Join(labels, "::")
}
