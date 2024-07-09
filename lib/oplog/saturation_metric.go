package oplog

import (
	"github.com/prometheus/client_golang/prometheus"
)

// IntervalMaxMetric is a prometheus metric that reports the maximum value reported to it within a configurable
// interval. These intervals are disjoint windows, and the *last* completed window is reported, if it immediately
// precedes the current one.
type SaturationMetric struct {
	// incoming vs outgoing delta
	delta *prometheus.Gauge
}
var metricLastCommandDuration = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "last_command_duration_seconds",
	Help:      "The round trip time in seconds of the most recent write to Redis.",
})

func NewSaturationMetric(server *metrics.Server) InstanceMetadataMetrics {
	delta := server.NewGauge(
		"saturation_delta",
		"delta between incoming and outgoing",
	)
	return SaturationMetric{
		&delta,
	}
}

func (metrics *SaturationMetric) recordIncoming() {
	if metrics.delta != nil {
		(*metrics.delta).Inc()
	}
}

func (metrics *SaturationMetric) recordOutgoing() {
	if metrics.delta != nil {
		(*metrics.delta).Dec()
	}
}
