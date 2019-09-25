package lib

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/require"
	"github.com/tulip/oplogtoredis/lib/oplog"
)

func TestPromHTTP(t *testing.T) {
	const (
		reportInterval = 100 * time.Millisecond
		namespace      = "test"
		subsystem      = "promhttp"
		name           = "test"
	)

	t.Parallel()

	req := require.New(t)

	reg := prometheus.NewRegistry()
	coll := oplog.NewIntervalMaxMetricVec(&oplog.IntervalMaxVecOpts{
		IntervalMaxOpts: oplog.IntervalMaxOpts{
			Opts: prometheus.Opts{
				Namespace:   namespace,
				Subsystem:   subsystem,
				Name:        name,
				Help:        "this is a test",
				ConstLabels: nil,
			},
			ReportInterval: reportInterval,
		},
	}, []string{"a"})
	reg.MustRegister(coll)

	fqName := prometheus.BuildFQName(namespace, subsystem, name)

	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer srv.Close()

	parser := &expfmt.TextParser{}

	update := func() map[string]*dto.MetricFamily {
		resp, err := http.Get(srv.URL)
		req.NoError(err)
		req.Equal(http.StatusOK, resp.StatusCode)

		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
		req.NoError(err)

		return metricFamilies
	}

	metricFamilies := update()
	req.Len(metricFamilies, 0)

	coll.Report(12.0, "b")

	metricFamilies = update()
	req.Len(metricFamilies, 0)

	time.Sleep(reportInterval)

	metricFamilies = update()
	req.Contains(metricFamilies, fqName)

	family := metricFamilies[fqName]
	req.Len(family.Metric, 1)

	mtc := family.Metric[0]
	req.NotNil(mtc.Gauge)
	req.NotNil(mtc.Gauge.Value)
	req.Equal(float64(12), *mtc.Gauge.Value)

	req.Len(mtc.Label, 1)
	req.NotNil(mtc.Label[0].Name)
	req.NotNil(mtc.Label[0].Value)
	req.Equal("a", *mtc.Label[0].Name)
	req.Equal("b", *mtc.Label[0].Value)

	time.Sleep(reportInterval)

	metricFamilies = update()
	req.Len(metricFamilies, 0)
}
