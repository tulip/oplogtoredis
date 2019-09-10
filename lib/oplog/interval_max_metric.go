package oplog

import (
	"strings"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/prometheus/client_golang/prometheus"
)

// Read this documentation https://golang.org/pkg/time/#hdr-Monotonic_Clocks before making changes to this file that
// affect the way time is handled.

// IntervalMaxMetric is a prometheus metric that reports the maximum value reported to it within a configurable
// interval. These intervals are disjoint windows, and the *last* completed window is reported, if it immediately
// precedes the current one.
type IntervalMaxMetric struct {
	desc        *prometheus.Desc
	opts        *IntervalMaxOpts
	labelValues []string

	// startBucket is the start of the initial bucket that this IntervalMaxMetric cares about. Time buckets are reckoned
	// in terms of interval-length offsets from this time; e.g. t0 = startBucket + 0.5*interval is in bucket 0, but
	// t1 = startBucket + 2*interval is in bucket 2.
	//
	// This value is intended to be used only as a source of monotonic time. It is set initially by synchronizing to
	// wall-clock time, truncated to our interval. This value will eventually drift from current wall-clock time, which
	// will cause the buckets to drift as well. The effects of this drift will be that the buckets do not line up nicely
	// with wall-clock time boundaries (e.g. a 1-minute interval may not break cleanly on the minute); and if the server
	// expects to scrape exactly on the interval, it may very rarely see a double-report. Neither of these problems
	// should be particularly concerning.
	startBucket time.Time

	lck sync.Mutex

	// currentMax contains the maximum record in the current time bucket, if any have been received.
	currentMax *maxRecord

	// previousMax contains the maximum record in the previous time bucket, if any were received. This value is what is
	// reported by Collect.
	previousMax *maxRecord
}

type maxRecord struct {
	value      float64
	timeBucket uint
}

// DefaultInterval is the default collection interval for IntervalMaxMetric.
const DefaultInterval = 1 * time.Minute

// IntervalMaxOpts are options for IntervalMaxMetric.
type IntervalMaxOpts struct {
	prometheus.Opts

	// ReportInterval is the interval by which reports will be bucketed. Default 1m.
	ReportInterval time.Duration

	nowFunc func() time.Time
}

// NewIntervalMaxMetric constructs a new IntervalMaxMetric.
func NewIntervalMaxMetric(opts *IntervalMaxOpts, labels []string, labelValues []string) *IntervalMaxMetric {
	if opts == nil {
		opts = &IntervalMaxOpts{}
	}

	if opts.ReportInterval == 0 {
		opts.ReportInterval = DefaultInterval
	}

	if opts.nowFunc == nil {
		opts.nowFunc = time.Now
	}

	now := opts.nowFunc()

	trunced := now.Truncate(opts.ReportInterval)
	diff := now.Sub(trunced)

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

		// maintain monotonic clock but truncate to interval boundary
		startBucket: now.Add(-diff),

		currentMax: nil,
	}
}

// Describe implements the prometheus.Collector interface.
func (c *IntervalMaxMetric) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.desc
}

// Collect implements the prometheus.Collector interface.
func (c *IntervalMaxMetric) Collect(mtcs chan<- prometheus.Metric) {
	c.lck.Lock()
	defer c.lck.Unlock()

	currentBucket := c.thisTimeBucket()
	c.rotate(currentBucket)

	if c.previousMax == nil {
		return
	}

	if currentBucket-(c.previousMax.timeBucket) != 1 {
		return
	}

	mtcs <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, c.previousMax.value, c.labelValues...)
}

// Report reports a value to the IntervalMaxMetric. If it is the greatest seen so far in the current window, it will be
// recorded as such until either another, greater value is reported, or the window ends (it will then be the
// value this IntervalMaxMetric returns when polled via Collect, until another window elapses).
func (c *IntervalMaxMetric) Report(value float64) {
	c.lck.Lock()
	defer c.lck.Unlock()

	thisTimeBucket := c.thisTimeBucket()
	c.rotate(thisTimeBucket)

	maxVal := &maxRecord{
		value:      value,
		timeBucket: thisTimeBucket,
	}

	if c.currentMax == nil {
		c.currentMax = maxVal
		return
	}

	if thisTimeBucket == c.currentMax.timeBucket {
		if c.currentMax.value < value {
			c.currentMax = maxVal
		}

		return
	}

	if thisTimeBucket > c.currentMax.timeBucket {
		c.currentMax = maxVal
		return
	}

	// this bucket is before previous bucket. this should be impossible because
	panic("interval max metric time traveled")
}

// pre: c is locked
func (c *IntervalMaxMetric) rotate(timeBucket uint) {
	if c.currentMax == nil {
		return
	}

	if timeBucket <= c.currentMax.timeBucket {
		return
	}

	// this behavior is expected by IntervalMaxMetricVec: must set currentMax nil if we've advanced a bucket
	c.previousMax = c.currentMax
	c.currentMax = nil
}

func (c *IntervalMaxMetric) thisTimeBucket() uint {
	return uint(c.opts.nowFunc().Sub(c.startBucket) / c.opts.ReportInterval)
}

// IntervalMaxVecOpts is options for IntervalMaxMetricVec.
type IntervalMaxVecOpts struct {
	IntervalMaxOpts

	// GCInterval is the interval on which the IntervalMaxMetricVec will clean up old state. This operation acquires
	// an exclusive lock on the entire metric, so this should be relatively long. Default 5s.
	GCInterval time.Duration
}

// DefaultMaxVecGCInterval is the default interval for cleaning up old state in IntervalMaxMetricVec.
const DefaultMaxVecGCInterval = 5 * time.Second

// IntervalMaxMetricVec is a Vec version of IntervalMaxMetric.
type IntervalMaxMetricVec struct {
	mp     sync.Map
	labels []string
	desc   *prometheus.Desc
	opts   *IntervalMaxVecOpts

	// lock locks mp. "read" access is more clearly interpreted as shared access, and "write" access as exclusive:
	// mp can be mutated with shared access, but gcs (and mutations to lastGc) must hold the lock exclusively.
	lock   sync.RWMutex
	lastGc time.Time
}

// NewIntervalMaxMetricVec constructs a new IntervalMaxMetricVec.
func NewIntervalMaxMetricVec(opts *IntervalMaxVecOpts, labels []string) *IntervalMaxMetricVec {
	if opts == nil {
		opts = &IntervalMaxVecOpts{}
	}

	if opts.GCInterval == 0 {
		opts.GCInterval = DefaultMaxVecGCInterval
	}

	if opts.nowFunc == nil {
		opts.nowFunc = time.Now
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

		lastGc: opts.nowFunc(),
	}
}

// Describe implements the prometheus.Collector interface.
func (c *IntervalMaxMetricVec) Describe(descs chan<- *prometheus.Desc) {
	descs <- c.desc
}

// Collect implements the prometheus.Collector interface.
func (c *IntervalMaxMetricVec) Collect(coll chan<- prometheus.Metric) {
	defer c.checkGc()

	c.lock.RLock()
	defer c.lock.RUnlock()

	c.mp.Range(func(_, v interface{}) bool {
		v.(*IntervalMaxMetric).Collect(coll)
		return true
	})
}

// Report reports a value to this collector. See IntervalMaxMetric.Report for details.
func (c *IntervalMaxMetricVec) Report(value float64, labelValues ...string) {
	defer c.checkGc()

	c.lock.RLock()
	defer c.lock.RUnlock()

	key := labelKey(labelValues)

	m, ok := c.mp.Load(key)
	if !ok {
		m, _ = c.mp.LoadOrStore(key, NewIntervalMaxMetric(&c.opts.IntervalMaxOpts, c.labels, labelValues))
	}

	m.(*IntervalMaxMetric).Report(value)
}

func labelKey(labels []string) string {
	return "imv::" + strings.Join(labels, "::")
}

func (c *IntervalMaxMetricVec) checkGc() {
	c.lock.RLock()
	timedOut := c.opts.nowFunc().Sub(c.lastGc) < c.opts.GCInterval
	c.lock.RUnlock()
	if timedOut {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.opts.nowFunc().Sub(c.lastGc) < c.opts.GCInterval { // another caller beat us to the punch
		return
	}

	c.gc()
}

// pre: exclusive lock acquired
func (c *IntervalMaxMetricVec) gc() {
	toEvict := mapset.NewSet()

	c.mp.Range(func(k, v interface{}) bool {
		m := v.(*IntervalMaxMetric)

		if m.currentMax != nil {
			return true
		}

		if m.previousMax == nil || m.thisTimeBucket()-m.previousMax.timeBucket > 1 {
			toEvict.Add(k)
		}

		return true
	})

	for k := range toEvict.Iter() {
		c.mp.Delete(k)
	}

	c.lastGc = c.opts.nowFunc()
}
