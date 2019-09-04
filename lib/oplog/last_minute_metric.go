package oplog

import (
	"strings"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"
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

	currentBucket := c.opts.thisTimeBucket()
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

	thisTimeBucket := c.opts.thisTimeBucket()
	c.rotate(thisTimeBucket)

	maxVal := &lastMax{
		value:        value,
		bucketedTime: thisTimeBucket,
	}

	if c.currentMax == nil {
		c.currentMax = maxVal
		return
	}

	if thisTimeBucket.Equal(c.currentMax.bucketedTime) {
		if c.currentMax.value < value {
			c.currentMax = maxVal
		}

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

	// this behavior is expected by IntervalMaxMetricVec: must set currentMax nil if we've advanced a bucket
	c.previousMax = c.currentMax
	c.currentMax = nil
}

func (opts IntervalMaxOpts) thisTimeBucket() time.Time {
	return time.Now().Truncate(opts.ReportInterval)
}

type IntervalMaxVecOpts struct {
	IntervalMaxOpts

	// GCInterval is the interval on which the IntervalMaxMetricVec will clean up old state. This operation acquires
	// an exclusive lock on the entire metric, so this should be relatively long. Default 5s.
	GCInterval time.Duration
}

const DefaultMaxVecGCInterval = 5 * time.Second

type IntervalMaxMetricVec struct {
	mp     sync.Map
	labels []string
	desc   *prometheus.Desc
	opts   IntervalMaxVecOpts

	// lock locks mp. "read" access is more clearly interpreted as shared access, and "write" access as exclusive:
	// mp can be mutated with shared access, but gcs (and mutations to lastGc) must hold the lock exclusively.
	lock   sync.RWMutex
	lastGc time.Time
}

func NewIntervalMaxMetricVec(opts IntervalMaxVecOpts, labels []string) *IntervalMaxMetricVec {
	if opts.GCInterval == 0 {
		opts.GCInterval = DefaultMaxVecGCInterval
	}

	return &IntervalMaxMetricVec{
		labels: labels,
		opts:   opts,
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
			opts.Help,
			labels,
			opts.ConstLabels,
		),

		lastGc: time.Now(),
	}
}

func (c *IntervalMaxMetricVec) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.desc
}

func (c *IntervalMaxMetricVec) Collect(coll chan<- prometheus.Metric) {
	defer c.checkGc()

	c.lock.RLock()
	defer c.lock.RUnlock()

	c.mp.Range(func(_, v interface{}) bool {
		v.(*IntervalMaxMetric).Collect(coll)
		return true
	})
}

func (c *IntervalMaxMetricVec) Report(value float64, labelValues ...string) {
	defer c.checkGc()

	c.lock.RLock()
	defer c.lock.RUnlock()

	key := labelKey(labelValues)

	m, ok := c.mp.Load(key)
	if !ok {
		m, _ = c.mp.LoadOrStore(key, NewIntervalMaxMetric(c.opts.IntervalMaxOpts, c.labels, labelValues))
	}

	m.(*IntervalMaxMetric).Report(value)
}

func labelKey(labels []string) string {
	return "imv::" + strings.Join(labels, "::")
}

func (c *IntervalMaxMetricVec) checkGc() {
	c.lock.RLock()
	timedOut := time.Since(c.lastGc) < c.opts.GCInterval
	c.lock.RUnlock()
	if timedOut {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if time.Since(c.lastGc) < c.opts.GCInterval { // another caller beat us to the bunch
		return
	}

	c.gc()
}

// pre: exclusive lock acquired
func (c *IntervalMaxMetricVec) gc() {
	toEvict := mapset.NewSet()

	c.mp.Range(func(k, v interface{}) bool {
		m := v.(*IntervalMaxMetric)

		if m.currentMax == nil && (m.previousMax == nil || time.Since(m.previousMax.bucketedTime) > 2*c.opts.ReportInterval) {
			toEvict.Add(k)
		}

		return true
	})

	for k := range toEvict.Iter() {
		c.mp.Delete(k)
	}

	c.lastGc = time.Now()
}
