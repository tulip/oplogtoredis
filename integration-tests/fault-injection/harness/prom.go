package harness

import (
	"fmt"
	"reflect"

	"github.com/kylelemons/godebug/pretty"

	promdata "github.com/prometheus/client_model/go"
)

// FindPromMetric is a helper to take the metrics scraped from oplogtoredis, and get a particular
// metric partition
func FindPromMetric(metrics map[string]*promdata.MetricFamily, name string, labels map[string]string) *promdata.Metric {
	metric, ok := metrics[name]
	if !ok {
		panic("No such metric: " + name)
	}

	for _, metricPartition := range metric.Metric {
		partitionLabels := map[string]string{}

		for _, label := range metricPartition.Label {
			partitionLabels[*label.Name] = *label.Value
		}

		if reflect.DeepEqual(labels, partitionLabels) {
			return metricPartition
		}
	}

	panic(fmt.Sprintf("Could not find desired metric. Desired labels: %#v. All partitions:\n%s",
		labels, pretty.Sprint(metric)))
}

// FindPromMetricCounter is like FindPromMetric, but then extracts an integer ounter value
func FindPromMetricCounter(metrics map[string]*promdata.MetricFamily, name string, labels map[string]string) int {
	val := FindPromMetric(metrics, name, labels).Counter.Value

	return int(*val)
}

// PromMetricOplogEntriesProcessed finds specifically the value of the otr_oplog_entries_received
// metric, and returns the value of the {database: "testdb", status: "processed"}
// partition
func PromMetricOplogEntriesProcessed(metrics map[string]*promdata.MetricFamily) int {
	return FindPromMetricCounter(metrics, "otr_oplog_entries_received", map[string]string{
		"database": "testdb",
		"status":   "processed",
	})
}
